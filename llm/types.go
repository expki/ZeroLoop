package llm

import "encoding/json"

// CompletionRequest is the request body for POST /completion (native llama.cpp endpoint)
type CompletionRequest struct {
	Prompt           any      `json:"prompt"`
	NPredict         int      `json:"n_predict,omitempty"`
	Temperature      float64  `json:"temperature,omitempty"`
	TopK             int      `json:"top_k,omitempty"`
	TopP             float64  `json:"top_p,omitempty"`
	MinP             float64  `json:"min_p,omitempty"`
	Stream           bool     `json:"stream"`
	Stop             []string `json:"stop,omitempty"`
	RepeatPenalty    float64  `json:"repeat_penalty,omitempty"`
	RepeatLastN      int      `json:"repeat_last_n,omitempty"`
	PresencePenalty  float64  `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64  `json:"frequency_penalty,omitempty"`
	Mirostat         int      `json:"mirostat,omitempty"`
	MirostatTau      float64  `json:"mirostat_tau,omitempty"`
	MirostatEta      float64  `json:"mirostat_eta,omitempty"`
	Grammar          string   `json:"grammar,omitempty"`
	JSONSchema       any      `json:"json_schema,omitempty"`
	Seed             int      `json:"seed,omitempty"`
	CachePrompt      *bool    `json:"cache_prompt,omitempty"`
	Samplers         []string `json:"samplers,omitempty"`
	IdSlot           int      `json:"id_slot,omitempty"`
	NKeep            int      `json:"n_keep,omitempty"`
	TypicalP         float64  `json:"typical_p,omitempty"`
}

// CompletionResponse is the non-streaming response from POST /completion
type CompletionResponse struct {
	Content            string          `json:"content"`
	Stop               bool            `json:"stop"`
	Model              string          `json:"model"`
	StopType           string          `json:"stop_type"`
	StoppingWord       string          `json:"stopping_word"`
	TokensEvaluated    int             `json:"tokens_evaluated"`
	TokensCached       int             `json:"tokens_cached"`
	Truncated          bool            `json:"truncated"`
	Timings            *Timings        `json:"timings,omitempty"`
	GenerationSettings json.RawMessage `json:"generation_settings,omitempty"`
}

// CompletionChunk is a streaming chunk from POST /completion
type CompletionChunk struct {
	Content  string   `json:"content"`
	Stop     bool     `json:"stop"`
	StopType string   `json:"stop_type,omitempty"`
	Timings  *Timings `json:"timings,omitempty"`
}

// Timings contains performance metrics from llama.cpp
type Timings struct {
	PromptN             int     `json:"prompt_n"`
	PromptMS            float64 `json:"prompt_ms"`
	PromptPerTokenMS    float64 `json:"prompt_per_token_ms"`
	PromptPerSecond     float64 `json:"prompt_per_second"`
	PredictedN          int     `json:"predicted_n"`
	PredictedMS         float64 `json:"predicted_ms"`
	PredictedPerTokenMS float64 `json:"predicted_per_token_ms"`
	PredictedPerSecond  float64 `json:"predicted_per_second"`
}

// ChatMessage represents a message in a chat conversation.
// Used for building history and for POST /apply-template.
type ChatMessage struct {
	Role       string     `json:"role"`
	Content    any        `json:"content"` // string or nil
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// Tool describes a tool available to the model
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a tool's function signature
type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

// ToolCall represents a tool call from the model
type ToolCall struct {
	Index    int              `json:"index"`
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction contains the function name and arguments for a tool call
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatCompletionResult is returned by the ChatCompletion convenience methods.
// It contains the parsed text content and any tool calls extracted from the response.
type ChatCompletionResult struct {
	Content   string
	ToolCalls []ToolCall
	Timings   *Timings
}

// StreamCallback is called for each streaming text chunk
type StreamCallback func(text string) error

// HealthResponse from GET /health
type HealthResponse struct {
	Status string `json:"status"`
}

// PropsResponse from GET /props
type PropsResponse struct {
	TotalSlots         int             `json:"total_slots"`
	ModelPath          string          `json:"model_path"`
	ChatTemplate       string          `json:"chat_template"`
	Modalities         map[string]bool `json:"modalities,omitempty"`
	DefaultGenSettings json.RawMessage `json:"default_generation_settings,omitempty"`
}

// TokenizeRequest for POST /tokenize
type TokenizeRequest struct {
	Content    string `json:"content"`
	AddSpecial bool   `json:"add_special,omitempty"`
	WithPieces bool   `json:"with_pieces,omitempty"`
}

// TokenizeResponse from POST /tokenize
type TokenizeResponse struct {
	Tokens json.RawMessage `json:"tokens"`
}

// DetokenizeRequest for POST /detokenize
type DetokenizeRequest struct {
	Tokens []int `json:"tokens"`
}

// DetokenizeResponse from POST /detokenize
type DetokenizeResponse struct {
	Content string `json:"content"`
}

// ChatCompletionResponse from POST /v1/chat/completions (non-streaming)
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
	Timings *Timings `json:"timings,omitempty"`
}

// Choice represents a single completion choice
type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatCompletionChunk is a streaming chunk from POST /v1/chat/completions
type ChatCompletionChunk struct {
	ID      string        `json:"id"`
	Choices []ChunkChoice `json:"choices"`
	Timings *Timings      `json:"timings,omitempty"`
}

// ChunkChoice represents a single choice in a streaming chunk
type ChunkChoice struct {
	Index        int        `json:"index"`
	Delta        ChunkDelta `json:"delta"`
	FinishReason string     `json:"finish_reason"`
}

// ChunkDelta contains the incremental content in a streaming chunk
type ChunkDelta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Usage contains token usage statistics
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// InfillRequest is the request body for POST /infill
type InfillRequest struct {
	InputPrefix string   `json:"input_prefix"`
	InputSuffix string   `json:"input_suffix"`
	NPredict    int      `json:"n_predict,omitempty"`
	Temperature float64  `json:"temperature,omitempty"`
	TopP        float64  `json:"top_p,omitempty"`
	Stop        []string `json:"stop,omitempty"`
	Stream      bool     `json:"stream"`
}

// InfillResponse from POST /infill
type InfillResponse struct {
	Content string `json:"content"`
}
