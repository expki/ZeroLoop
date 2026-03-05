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

func TestModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("expected /v1/models, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(ModelListResponse{
			Object: "list",
			Data:   []ModelEntry{{ID: "test-model", Object: "model"}},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	models, err := c.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models.Data) != 1 || models.Data[0].ID != "test-model" {
		t.Errorf("unexpected models: %+v", models)
	}
}

func TestChatCompletion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatCompletionRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Stream {
			t.Error("expected stream=false for non-streaming request")
		}
		if len(req.Messages) == 0 {
			t.Error("expected at least one message")
		}
		json.NewEncoder(w).Encode(ChatCompletionResponse{
			ID:     "test-id",
			Object: "chat.completion",
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
	resp, err := c.ChatCompletion(context.Background(), &ChatCompletionRequest{
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	content, ok := resp.Choices[0].Message.Content.(string)
	if !ok || content != "Hello!" {
		t.Errorf("unexpected content: %v", resp.Choices[0].Message.Content)
	}
}

func TestChatCompletionStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		chunks := []string{"Hello", " ", "World"}
		for _, chunk := range chunks {
			content := chunk
			data := ChatCompletionChunk{
				ID:     "test",
				Object: "chat.completion.chunk",
				Choices: []ChunkChoice{
					{
						Index: 0,
						Delta: ChunkDelta{Content: &content},
					},
				},
			}
			b, _ := json.Marshal(data)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	var collected string
	var chunkCount int
	err := c.ChatCompletionStream(context.Background(), &ChatCompletionRequest{
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	}, func(chunk ChatCompletionChunk) error {
		chunkCount++
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != nil {
			collected += *chunk.Choices[0].Delta.Content
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if chunkCount != 3 {
		t.Errorf("expected 3 chunks, got %d", chunkCount)
	}
	if collected != "Hello World" {
		t.Errorf("expected 'Hello World', got '%s'", collected)
	}
}

func TestChatCompletionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "bad request"}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.ChatCompletion(context.Background(), &ChatCompletionRequest{
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Error("expected error for bad request")
	}
}

func TestChatCompletionStreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`service unavailable`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	err := c.ChatCompletionStream(context.Background(), &ChatCompletionRequest{
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	}, func(chunk ChatCompletionChunk) error {
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
	// Health doesn't check status code, it just tries to decode JSON.
	// With invalid JSON, it should return an error.
	if err != nil {
		// Expected: non-JSON body causes decode error
		return
	}
	// If it somehow decoded, status should not be "ok"
	if health.Status == "ok" {
		t.Error("expected non-ok status from 503 response")
	}
}
