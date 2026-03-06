package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/expki/ZeroLoop.git/llm"
	"github.com/expki/ZeroLoop.git/logger"
)

// ErrPaused is returned when the agent loop is interrupted by a user pause.
var ErrPaused = errors.New("agent paused")

// ErrCancelled is returned when the agent loop is interrupted by a user cancel.
var ErrCancelled = errors.New("agent cancelled")

// LogEntry represents a log item sent to the UI
type LogEntry struct {
	Type    string            `json:"type"`    // user, agent, response, tool, code_exe, error, info, warning, progress
	Heading string            `json:"heading"`
	Content string            `json:"content"`
	Kvps    map[string]string `json:"kvps,omitempty"`
	AgentNo int               `json:"agentno"`
	Stream  bool              `json:"stream"` // true if this is a streaming chunk (append to previous)
}

// LogCallback is called when the agent produces log output
type LogCallback func(entry LogEntry)

// Agent represents a single agent instance
type Agent struct {
	Number  int
	AgentID string // identifies which agent session this agent belongs to
	Profile string // agent profile name (default, developer, researcher, hacker)
	History []llm.ChatMessage
	Tools   *ToolRegistry
	LLM     *llm.Client
	OnLog   LogCallback
	Parent  *Agent

	// Project context (set when chat belongs to a project)
	ProjectID         string                // project ID for scoping file operations
	ProjectDir        string                // absolute path to project folder
	FileEventCallback func(event FileEvent) // broadcasts file changes via WS

	// Intervention allows users to send messages while the agent is running
	intervention     string
	interventionMu   sync.Mutex
	interventionCond *sync.Cond

	// Secrets to mask in output
	Secrets map[string]string // name -> value

	// Pause state (atomic)
	paused    int32
	cancelled int32

	// Tool context cancellation (separate from main context so pause doesn't kill tools)
	toolCancelMu sync.Mutex
	toolCancel   context.CancelFunc

	mu sync.Mutex
}

// SetPaused sets the agent's paused flag atomically.
func (a *Agent) SetPaused(p bool) {
	if p {
		atomic.StoreInt32(&a.paused, 1)
	} else {
		atomic.StoreInt32(&a.paused, 0)
	}
}

// IsPaused returns whether the agent is currently paused.
func (a *Agent) IsPaused() bool {
	return atomic.LoadInt32(&a.paused) == 1
}

// SetCancelled sets the agent's cancelled flag atomically.
func (a *Agent) SetCancelled(c bool) {
	if c {
		atomic.StoreInt32(&a.cancelled, 1)
	} else {
		atomic.StoreInt32(&a.cancelled, 0)
	}
}

// IsCancelled returns whether the agent is currently cancelled.
func (a *Agent) IsCancelled() bool {
	return atomic.LoadInt32(&a.cancelled) == 1
}

// CancelTools cancels any in-progress tool execution context.
// Used by cancel/clear to stop tools immediately. Pause does NOT call this.
func (a *Agent) CancelTools() {
	a.toolCancelMu.Lock()
	if a.toolCancel != nil {
		a.toolCancel()
	}
	a.toolCancelMu.Unlock()
}

// New creates a new root agent
func New(llmClient *llm.Client, tools *ToolRegistry, onLog LogCallback) *Agent {
	a := &Agent{
		Number:  0,
		History: []llm.ChatMessage{},
		Tools:   tools,
		LLM:     llmClient,
		OnLog:   onLog,
		Secrets: make(map[string]string),
	}
	a.interventionCond = sync.NewCond(&a.interventionMu)
	return a
}

// NewSubordinate creates a child agent
func (a *Agent) NewSubordinate() *Agent {
	sub := &Agent{
		Number:            a.Number + 1,
		AgentID:           a.AgentID,
		Profile:           a.Profile,
		History:           []llm.ChatMessage{},
		Tools:             a.Tools,
		LLM:               a.LLM,
		OnLog:             a.OnLog,
		Parent:            a,
		Secrets:           a.Secrets,
		ProjectID:         a.ProjectID,
		ProjectDir:        a.ProjectDir,
		FileEventCallback: a.FileEventCallback,
	}
	sub.interventionCond = sync.NewCond(&sub.interventionMu)
	return sub
}

// Intervene injects a user message into the running agent loop
func (a *Agent) Intervene(message string) {
	a.interventionMu.Lock()
	a.intervention = message
	a.interventionMu.Unlock()
}

// checkIntervention checks if there's a pending intervention and returns it
func (a *Agent) checkIntervention() string {
	a.interventionMu.Lock()
	msg := a.intervention
	a.intervention = ""
	a.interventionMu.Unlock()
	return msg
}

const (
	maxHistoryMessages    = 100
	compressionThreshold  = 50 // Trigger compression when history exceeds this
	compressionKeepRecent = 20 // Keep this many recent messages after compression
)

// compressHistory uses the LLM to summarize older messages into a compact form.
// Falls back to simple pruning if LLM call fails.
func (a *Agent) compressHistory(ctx context.Context) {
	if len(a.History) <= compressionThreshold {
		return
	}

	a.Log(LogEntry{
		Type:    "info",
		Heading: "Compressing History",
		Content: fmt.Sprintf("History has %d messages, compressing...", len(a.History)),
		AgentNo: a.Number,
	})

	// Split history: first message + messages to compress + recent messages to keep
	firstMsg := a.History[0]
	toCompress := a.History[1 : len(a.History)-compressionKeepRecent]
	recentMsgs := a.History[len(a.History)-compressionKeepRecent:]

	// Build compression prompt
	var sb strings.Builder
	sb.WriteString("Summarize the following conversation history into a concise summary. ")
	sb.WriteString("Preserve key facts, decisions, tool results, and context needed to continue the conversation. ")
	sb.WriteString("Format as a structured summary with bullet points.\n\n")
	for _, msg := range toCompress {
		role := msg.Role
		content := ""
		switch v := msg.Content.(type) {
		case string:
			content = v
		case nil:
			content = "(tool call)"
		}
		if content != "" {
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			sb.WriteString(fmt.Sprintf("[%s]: %s\n", role, content))
		}
	}

	result, err := a.LLM.ChatCompletion(ctx, []llm.ChatMessage{
		{Role: "system", Content: "You summarize conversation histories concisely. Output ONLY the summary."},
		{Role: "user", Content: sb.String()},
	}, nil, nil)
	if err != nil {
		// Fallback to simple pruning
		a.pruneHistory()
		return
	}

	summary := result.Content
	if summary == "" {
		a.pruneHistory()
		return
	}

	// Rebuild history: first message + summary + recent messages
	compressed := make([]llm.ChatMessage, 0, 2+len(recentMsgs))
	compressed = append(compressed, firstMsg)
	compressed = append(compressed, llm.ChatMessage{
		Role:    "user",
		Content: fmt.Sprintf("[CONVERSATION SUMMARY]\n%s\n[END SUMMARY - conversation continues below]", summary),
	})
	compressed = append(compressed, recentMsgs...)
	a.History = compressed

	a.Log(LogEntry{
		Type:    "info",
		Heading: "History Compressed",
		Content: fmt.Sprintf("Compressed %d messages into summary. History now has %d messages.", len(toCompress), len(a.History)),
		AgentNo: a.Number,
	})
}

// pruneHistory trims history to stay within context limits (fallback).
func (a *Agent) pruneHistory() {
	if len(a.History) <= maxHistoryMessages {
		return
	}
	pruned := make([]llm.ChatMessage, 0, maxHistoryMessages)
	pruned = append(pruned, a.History[0])
	pruned = append(pruned, a.History[len(a.History)-(maxHistoryMessages-1):]...)
	a.History = pruned
}

// maskSecrets replaces secret values with placeholders in the given text
func (a *Agent) maskSecrets(text string) string {
	for name, value := range a.Secrets {
		if value != "" && len(value) > 3 {
			text = strings.ReplaceAll(text, value, fmt.Sprintf("***%s***", name))
		}
	}
	return text
}

// Run executes the agent loop for a user message.
// It adds the user message to history, then enters the think→execute→observe loop.
// Returns the final response content.
func (a *Agent) Run(ctx context.Context, userMessage string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Add user message to history
	a.History = append(a.History, llm.ChatMessage{
		Role:    "user",
		Content: userMessage,
	})

	return a.runLoop(ctx)
}

// Continue re-enters the agent loop without adding a new user message.
// Used after a pause to resume processing from the current history state.
func (a *Agent) Continue(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.SetPaused(false)
	return a.runLoop(ctx)
}

// runLoop is the core agent loop: think → execute → observe → repeat.
func (a *Agent) runLoop(ctx context.Context) (string, error) {
	// Build system prompt (profile-aware)
	systemPrompt := SystemPromptWithProfile(a.Number, a.Profile, a.ProjectDir, a.Tools.All())

	// Track last response for repeat detection
	var lastResponse string
	var lastToolSig string
	toolRepeatCount := 0
	const maxToolRepeats = 3

	// Agent loop: think → execute → observe → repeat
	maxIterations := 25
	for i := 0; i < maxIterations; i++ {
		select {
		case <-ctx.Done():
			if a.IsCancelled() {
				return "", ErrCancelled
			}
			if a.IsPaused() {
				return "", ErrPaused
			}
			return "", ctx.Err()
		default:
		}

		// Check for user intervention
		if intervention := a.checkIntervention(); intervention != "" {
			a.Log(LogEntry{
				Type:    "info",
				Heading: "User Intervention",
				Content: intervention,
				AgentNo: a.Number,
			})
			a.History = append(a.History, llm.ChatMessage{
				Role:    "user",
				Content: fmt.Sprintf("[USER INTERVENTION]: %s", intervention),
			})
		}

		// Compress history if needed (uses LLM)
		a.compressHistory(ctx)

		// Fallback prune if compression didn't reduce enough
		a.pruneHistory()

		// Build messages for LLM
		messages := make([]llm.ChatMessage, 0, len(a.History)+1)
		messages = append(messages, llm.ChatMessage{
			Role:    "system",
			Content: systemPrompt,
		})
		messages = append(messages, a.History...)

		// Log that we're thinking (only on subsequent iterations)
		if i > 0 {
			a.Log(LogEntry{
				Type:    "agent",
				Heading: "Thinking",
				Content: fmt.Sprintf("Iteration %d", i+1),
				AgentNo: a.Number,
			})
		}

		// Call LLM with streaming via /v1/chat/completions
		firstChunk := true
		var streamAccum strings.Builder
		result, err := a.LLM.ChatCompletionStream(ctx, messages, a.Tools.ToLLMTools(), nil, func(text string) error {
			streamAccum.WriteString(text)
			maskedContent := a.maskSecrets(text)
			a.Log(LogEntry{
				Type:    "agent",
				Heading: "Thinking",
				Content: maskedContent,
				AgentNo: a.Number,
				Stream:  !firstChunk,
			})
			firstChunk = false
			return nil
		})

		if err != nil {
			// If cancelled or paused, return quietly without logging errors
			if a.IsCancelled() {
				return "", ErrCancelled
			}
			if a.IsPaused() {
				// Save partial response to history so resume can continue
				if partial := streamAccum.String(); partial != "" {
					a.History = append(a.History, llm.ChatMessage{
						Role:    "assistant",
						Content: partial,
					})
					a.History = append(a.History, llm.ChatMessage{
						Role:    "user",
						Content: "[SYSTEM: Your previous response was interrupted. Continue from exactly where you left off. Do not repeat what you already said.]",
					})
				}
				return "", ErrPaused
			}
			a.Log(LogEntry{
				Type:    "error",
				Heading: "LLM Error",
				Content: a.maskSecrets(err.Error()),
				AgentNo: a.Number,
			})
			return "", fmt.Errorf("llm stream error: %w", err)
		}

		responseContent := result.Content
		toolCalls := result.ToolCalls

		// Repeat detection
		if responseContent != "" && responseContent == lastResponse {
			a.Log(LogEntry{
				Type:    "warning",
				Heading: "Repeat Detected",
				Content: "Agent produced the same response twice. Breaking loop.",
				AgentNo: a.Number,
			})
			a.History = append(a.History, llm.ChatMessage{
				Role:    "user",
				Content: "WARNING: You are repeating yourself. Please provide a different response or use the response tool to deliver your answer.",
			})
			lastResponse = ""
			continue
		}
		lastResponse = responseContent

		// Tool call repeat detection: build a signature from tool names + arguments
		if len(toolCalls) > 0 {
			var sigBuilder strings.Builder
			for _, tc := range toolCalls {
				sigBuilder.WriteString(tc.Function.Name)
				sigBuilder.WriteString(":")
				sigBuilder.WriteString(tc.Function.Arguments)
				sigBuilder.WriteString(";")
			}
			toolSig := sigBuilder.String()
			if toolSig == lastToolSig {
				toolRepeatCount++
				if toolRepeatCount >= maxToolRepeats {
					a.Log(LogEntry{
						Type:    "warning",
						Heading: "Tool Loop Detected",
						Content: fmt.Sprintf("Agent called the same tool(s) %d times in a row with identical arguments. Breaking loop.", toolRepeatCount+1),
						AgentNo: a.Number,
					})
					// Add a warning to history and force a different approach
					a.History = append(a.History, llm.ChatMessage{
						Role:      "assistant",
						Content:   nil,
						ToolCalls: toolCalls,
					})
					for _, tc := range toolCalls {
						a.History = append(a.History, llm.ChatMessage{
							Role:       "tool",
							Content:    "ERROR: You have called this tool repeatedly with the same arguments and it keeps failing. Stop retrying and use the response tool to explain the issue to the user.",
							ToolCallID: tc.ID,
						})
					}
					lastToolSig = ""
					toolRepeatCount = 0
					continue
				}
			} else {
				lastToolSig = toolSig
				toolRepeatCount = 0
			}
		}

		// Add assistant response to history
		histMsg := llm.ChatMessage{
			Role:      "assistant",
			Content:   responseContent,
			ToolCalls: toolCalls,
		}
		if len(toolCalls) > 0 && responseContent == "" {
			histMsg.Content = nil
		}
		a.History = append(a.History, histMsg)

		// Process tool calls
		if len(toolCalls) > 0 {
			breakLoop := false
			var finalResponse string

			// Create a separate tool context NOT tied to the main ctx.
			// Pause cancels main ctx (to stop LLM streaming) but tools should finish.
			// Cancel/Clear calls CancelTools() to stop this context too.
			toolCtx, toolCancelFn := context.WithTimeout(context.Background(), 5*time.Minute)
			a.toolCancelMu.Lock()
			a.toolCancel = toolCancelFn
			a.toolCancelMu.Unlock()

			for tcIdx, tc := range toolCalls {
				// Check for cancel between tool calls (cancel stops tools, pause does not)
				if a.IsCancelled() {
					for _, remaining := range toolCalls[tcIdx:] {
						a.History = append(a.History, llm.ChatMessage{
							Role:       "tool",
							Content:    "[Tool execution cancelled by user]",
							ToolCallID: remaining.ID,
						})
					}
					toolCancelFn()
					return "", ErrCancelled
				}

				// Check for pause between tool calls (let current tool complete, stop before next)
				if a.IsPaused() {
					for _, remaining := range toolCalls[tcIdx:] {
						a.History = append(a.History, llm.ChatMessage{
							Role:       "tool",
							Content:    "[Tool execution paused by user — will retry on resume]",
							ToolCallID: remaining.ID,
						})
					}
					toolCancelFn()
					return "", ErrPaused
				}

				// Check for intervention between tool calls
				if intervention := a.checkIntervention(); intervention != "" {
					a.Log(LogEntry{
						Type:    "info",
						Heading: "User Intervention",
						Content: intervention,
						AgentNo: a.Number,
					})
					a.History = append(a.History, llm.ChatMessage{
						Role:    "user",
						Content: fmt.Sprintf("[USER INTERVENTION]: %s", intervention),
					})
				}

				tool, ok := a.Tools.Get(tc.Function.Name)
				if !ok {
					errMsg := fmt.Sprintf("Unknown tool: %s", tc.Function.Name)
					a.Log(LogEntry{
						Type:    "error",
						Heading: "Tool Error",
						Content: errMsg,
						AgentNo: a.Number,
					})
					a.History = append(a.History, llm.ChatMessage{
						Role:       "tool",
						Content:    errMsg,
						ToolCallID: tc.ID,
					})
					continue
				}

				// Parse arguments
				var args map[string]any
				if tc.Function.Arguments != "" {
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
						args = map[string]any{"raw": tc.Function.Arguments}
					}
				}

				// Log tool execution
				kvps := map[string]string{"tool_name": tc.Function.Name}
				for k, v := range args {
					kvps[k] = fmt.Sprintf("%v", v)
				}
				a.Log(LogEntry{
					Type:    "tool",
					Heading: fmt.Sprintf("Using %s", tc.Function.Name),
					Content: a.maskSecrets(tc.Function.Arguments),
					Kvps:    kvps,
					AgentNo: a.Number,
				})

				// Execute tool with separate context (not cancelled by pause)
				result, err := tool.Execute(toolCtx, a, args)
				if err != nil {
					// Cancel interrupted the tool
					if a.IsCancelled() {
						a.History = append(a.History, llm.ChatMessage{
							Role:       "tool",
							Content:    "[Tool execution cancelled by user]",
							ToolCallID: tc.ID,
						})
						for _, remaining := range toolCalls[tcIdx+1:] {
							a.History = append(a.History, llm.ChatMessage{
								Role:       "tool",
								Content:    "[Tool execution cancelled by user]",
								ToolCallID: remaining.ID,
							})
						}
						toolCancelFn()
						return "", ErrCancelled
					}
					errMsg := fmt.Sprintf("Tool error: %s", a.maskSecrets(err.Error()))
					a.Log(LogEntry{
						Type:    "error",
						Heading: "Tool Error",
						Content: errMsg,
						AgentNo: a.Number,
					})
					a.History = append(a.History, llm.ChatMessage{
						Role:       "tool",
						Content:    errMsg,
						ToolCallID: tc.ID,
					})
					continue
				}

				// Mask secrets in tool result
				maskedMessage := a.maskSecrets(result.Message)

				// Add tool result to history
				a.History = append(a.History, llm.ChatMessage{
					Role:       "tool",
					Content:    result.Message,
					ToolCallID: tc.ID,
				})

				if result.BreakLoop {
					breakLoop = true
					finalResponse = maskedMessage
				}
			}

			// Clean up tool context
			toolCancelFn()
			a.toolCancelMu.Lock()
			a.toolCancel = nil
			a.toolCancelMu.Unlock()

			// Check pause after all tools completed
			if a.IsPaused() {
				return "", ErrPaused
			}

			if breakLoop {
				a.Log(LogEntry{
					Type:    "response",
					Heading: "Response",
					Content: finalResponse,
					AgentNo: a.Number,
				})
				return finalResponse, nil
			}

			continue
		}

		// No tool calls — direct response fallback
		if responseContent != "" {
			a.Log(LogEntry{
				Type:    "response",
				Heading: "Response",
				Content: a.maskSecrets(responseContent),
				AgentNo: a.Number,
			})
			return responseContent, nil
		}
	}

	return "", fmt.Errorf("agent reached maximum iterations (%d)", maxIterations)
}

// Log sends a log entry to the registered callback or falls back to the logger.
func (a *Agent) Log(entry LogEntry) {
	if a.OnLog != nil {
		a.OnLog(entry)
	} else {
		logger.Log.Debugw("agent log", "type", entry.Type, "heading", entry.Heading)
	}
}
