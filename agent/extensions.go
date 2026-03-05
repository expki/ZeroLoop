package agent

import (
	"context"
	"sort"
	"sync"

	"github.com/expki/ZeroLoop.git/logger"
)

// ExtensionPoint identifies where in the agent loop an extension runs
type ExtensionPoint string

const (
	ExtMonologueStart       ExtensionPoint = "monologue_start"
	ExtMonologueEnd         ExtensionPoint = "monologue_end"
	ExtMessageLoopStart     ExtensionPoint = "message_loop_start"
	ExtMessageLoopEnd       ExtensionPoint = "message_loop_end"
	ExtBeforeMainLLMCall    ExtensionPoint = "before_main_llm_call"
	ExtAfterMainLLMCall     ExtensionPoint = "after_main_llm_call"
	ExtToolExecuteBefore    ExtensionPoint = "tool_execute_before"
	ExtToolExecuteAfter     ExtensionPoint = "tool_execute_after"
	ExtSystemPrompt         ExtensionPoint = "system_prompt"
	ExtHistoryAddBefore     ExtensionPoint = "hist_add_before"
	ExtResponseStreamChunk  ExtensionPoint = "response_stream_chunk"
	ExtResponseStreamEnd    ExtensionPoint = "response_stream_end"
	ExtProcessChainEnd      ExtensionPoint = "process_chain_end"
)

// ExtensionContext provides data to extensions at each hook point
type ExtensionContext struct {
	Agent     *Agent
	Iteration int
	ToolName  string
	ToolArgs  map[string]any
	Content   string // mutable - extensions can modify this
}

// Extension is a function that runs at a specific hook point
type Extension struct {
	Name     string
	Priority int // lower runs first
	Fn       func(ctx context.Context, ec *ExtensionContext) error
}

// ExtensionRegistry manages registered extensions
type ExtensionRegistry struct {
	extensions map[ExtensionPoint][]Extension
	mu         sync.RWMutex
}

// NewExtensionRegistry creates a new extension registry
func NewExtensionRegistry() *ExtensionRegistry {
	return &ExtensionRegistry{
		extensions: make(map[ExtensionPoint][]Extension),
	}
}

// Register adds an extension at the given hook point
func (r *ExtensionRegistry) Register(point ExtensionPoint, ext Extension) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.extensions[point] = append(r.extensions[point], ext)
	// Sort by priority
	sort.Slice(r.extensions[point], func(i, j int) bool {
		return r.extensions[point][i].Priority < r.extensions[point][j].Priority
	})
}

// Run executes all extensions at the given hook point in priority order
func (r *ExtensionRegistry) Run(ctx context.Context, point ExtensionPoint, ec *ExtensionContext) error {
	r.mu.RLock()
	exts := r.extensions[point]
	r.mu.RUnlock()

	for _, ext := range exts {
		if err := ext.Fn(ctx, ec); err != nil {
			if logger.Log != nil {
				logger.Log.Warnw("extension error",
					"point", string(point),
					"extension", ext.Name,
					"error", err,
				)
			}
			// Continue with other extensions unless it's a critical error
		}
	}
	return nil
}

// Has returns true if any extensions are registered for the given point
func (r *ExtensionRegistry) Has(point ExtensionPoint) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.extensions[point]) > 0
}

// defaultExtensions is the global extension registry
var defaultExtensions = NewExtensionRegistry()

// RegisterExtension registers an extension in the global registry
func RegisterExtension(point ExtensionPoint, ext Extension) {
	defaultExtensions.Register(point, ext)
}

// RunExtensions runs all extensions at the given hook point using the global registry
func RunExtensions(ctx context.Context, point ExtensionPoint, ec *ExtensionContext) error {
	return defaultExtensions.Run(ctx, point, ec)
}

func init() {
	// Register default built-in extensions

	// Monologue start: log iteration
	RegisterExtension(ExtMessageLoopStart, Extension{
		Name:     "iteration_tracker",
		Priority: 10,
		Fn: func(ctx context.Context, ec *ExtensionContext) error {
			if ec.Iteration > 1 && logger.Log != nil {
				logger.Log.Debugw("agent iteration", "agent", ec.Agent.Number, "iteration", ec.Iteration)
			}
			return nil
		},
	})
}
