package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("http://example.com/")
	if c.baseURL != "http://example.com" {
		t.Errorf("expected trimmed URL, got %s", c.baseURL)
	}
}

func TestHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("expected /health, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	health, err := c.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if health.Status != "ok" {
		t.Errorf("expected status ok, got %s", health.Status)
	}
}

func TestProps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/props" {
			t.Errorf("expected /props, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(PropsResponse{
			TotalSlots: 2,
			ModelPath:  "/models/test.gguf",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	props, err := c.Props(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if props.TotalSlots != 2 {
		t.Errorf("expected 2 slots, got %d", props.TotalSlots)
	}
	if props.ModelPath != "/models/test.gguf" {
		t.Errorf("expected model path, got %s", props.ModelPath)
	}
}

func TestCompletion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/completion" {
			t.Errorf("expected /completion, got %s", r.URL.Path)
		}
		var req CompletionRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Stream {
			t.Error("expected stream=false for non-streaming request")
		}
		json.NewEncoder(w).Encode(CompletionResponse{
			Content:  "Hello World!",
			Stop:     true,
			StopType: "eos",
			Model:    "test-model",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	resp, err := c.Completion(context.Background(), &CompletionRequest{
		Prompt: "Say hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Hello World!" {
		t.Errorf("expected 'Hello World!', got '%s'", resp.Content)
	}
}

func TestCompletionStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		chunks := []CompletionChunk{
			{Content: "Hello", Stop: false},
			{Content: " ", Stop: false},
			{Content: "World", Stop: false},
			{Content: "", Stop: true, StopType: "eos"},
		}
		for _, chunk := range chunks {
			b, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	var collected string
	var chunkCount int
	err := c.CompletionStream(context.Background(), &CompletionRequest{
		Prompt: "Say hello",
	}, func(chunk CompletionChunk) error {
		chunkCount++
		collected += chunk.Content
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if chunkCount != 4 {
		t.Errorf("expected 4 chunks, got %d", chunkCount)
	}
	if collected != "Hello World" {
		t.Errorf("expected 'Hello World', got '%s'", collected)
	}
}

func TestChatCompletion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(ChatCompletionResponse{
			Choices: []Choice{
				{
					Index: 0,
					Message: ChatMessage{
						Role:    "assistant",
						Content: "Hello!",
					},
					FinishReason: "stop",
				},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	result, err := c.ChatCompletion(context.Background(), []ChatMessage{
		{Role: "user", Content: "Hi"},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "Hello!" {
		t.Errorf("expected 'Hello!', got '%s'", result.Content)
	}
}

func TestChatCompletionStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		chunks := []ChatCompletionChunk{
			{Choices: []ChunkChoice{{Delta: ChunkDelta{Content: "Hello"}}}},
			{Choices: []ChunkChoice{{Delta: ChunkDelta{Content: " World"}}}},
		}
		for _, chunk := range chunks {
			b, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	var collected string
	result, err := c.ChatCompletionStream(context.Background(), []ChatMessage{
		{Role: "user", Content: "Hi"},
	}, nil, nil, func(text string) error {
		collected += text
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if collected != "Hello World" {
		t.Errorf("expected 'Hello World' from callback, got '%s'", collected)
	}
	if result.Content != "Hello World" {
		t.Errorf("expected 'Hello World' from result, got '%s'", result.Content)
	}
}

func TestChatCompletionWithToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(ChatCompletionResponse{
			Choices: []Choice{
				{
					Index: 0,
					Message: ChatMessage{
						Role:    "assistant",
						Content: "I'll search for that.",
						ToolCalls: []ToolCall{
							{
								Index: 0,
								ID:    "call_123",
								Type:  "function",
								Function: ToolCallFunction{
									Name:      "web_search",
									Arguments: `{"query": "golang best practices"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	result, err := c.ChatCompletion(context.Background(), []ChatMessage{
		{Role: "user", Content: "Search for golang best practices"},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "I'll search for that." {
		t.Errorf("unexpected content: %q", result.Content)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Function.Name != "web_search" {
		t.Errorf("expected 'web_search', got %q", result.ToolCalls[0].Function.Name)
	}
}

func TestChatCompletionStreamWithToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		chunks := []ChatCompletionChunk{
			{Choices: []ChunkChoice{{Delta: ChunkDelta{
				ToolCalls: []ToolCall{{Index: 0, ID: "call_1", Type: "function", Function: ToolCallFunction{Name: "web_search", Arguments: ""}}},
			}}}},
			{Choices: []ChunkChoice{{Delta: ChunkDelta{
				ToolCalls: []ToolCall{{Index: 0, Function: ToolCallFunction{Arguments: `{"query":`}}},
			}}}},
			{Choices: []ChunkChoice{{Delta: ChunkDelta{
				ToolCalls: []ToolCall{{Index: 0, Function: ToolCallFunction{Arguments: ` "test"}`}}},
			}}}},
		}
		for _, chunk := range chunks {
			b, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	result, err := c.ChatCompletionStream(context.Background(), []ChatMessage{
		{Role: "user", Content: "Search"},
	}, nil, nil, func(text string) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Function.Name != "web_search" {
		t.Errorf("expected 'web_search', got %q", result.ToolCalls[0].Function.Name)
	}
	if result.ToolCalls[0].Function.Arguments != `{"query": "test"}` {
		t.Errorf("unexpected arguments: %q", result.ToolCalls[0].Function.Arguments)
	}
	if result.ToolCalls[0].ID != "call_1" {
		t.Errorf("expected id 'call_1', got %q", result.ToolCalls[0].ID)
	}
}

func TestCompletionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "bad request"}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.Completion(context.Background(), &CompletionRequest{
		Prompt: "test",
	})
	if err == nil {
		t.Error("expected error for bad request")
	}
}

func TestCompletionStreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`service unavailable`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	err := c.CompletionStream(context.Background(), &CompletionRequest{
		Prompt: "test",
	}, func(chunk CompletionChunk) error {
		t.Error("callback should not be called on error")
		return nil
	})
	if err == nil {
		t.Error("expected error for 503 response")
	}
}

func TestHealthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	health, err := c.Health(context.Background())
	if err != nil {
		return
	}
	if health.Status == "ok" {
		t.Error("expected non-ok status from 503 response")
	}
}
