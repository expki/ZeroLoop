package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

// Health checks the server health via GET /health
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, err
	}
	return &health, nil
}

// Props returns server properties via GET /props (replaces Models)
func (c *Client) Props(ctx context.Context) (*PropsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/props", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var props PropsResponse
	if err := json.NewDecoder(resp.Body).Decode(&props); err != nil {
		return nil, err
	}
	return &props, nil
}

// ApplyTemplate converts chat messages to a prompt string via POST /apply-template
func (c *Client) ApplyTemplate(ctx context.Context, messages []ChatMessage, tools []Tool) (string, error) {
	payload := map[string]any{
		"messages": messages,
	}
	if len(tools) > 0 {
		payload["tools"] = tools
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/apply-template", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("apply-template failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Prompt, nil
}

// Completion sends a non-streaming completion request via POST /completion
func (c *Client) Completion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	req.Stream = false
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/completion", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("completion request failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result CompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CompletionStream sends a streaming completion request via POST /completion
func (c *Client) CompletionStream(ctx context.Context, req *CompletionRequest, callback func(CompletionChunk) error) error {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/completion", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("completion stream failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		var chunk CompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if err := callback(chunk); err != nil {
			return err
		}
		if chunk.Stop {
			break
		}
	}
	return scanner.Err()
}

// ChatCompletion sends a non-streaming chat completion request via POST /v1/chat/completions.
// Tool calls are returned as structured data from the server.
func (c *Client) ChatCompletion(ctx context.Context, messages []ChatMessage, tools []Tool, opts *CompletionRequest) (*ChatCompletionResult, error) {
	reqBody := map[string]any{
		"messages": messages,
		"stream":   false,
	}
	if len(tools) > 0 {
		reqBody["tools"] = tools
	}
	if opts != nil {
		applyChatOpts(reqBody, opts)
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chat completion failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, err
	}

	if len(chatResp.Choices) == 0 {
		return &ChatCompletionResult{}, nil
	}

	choice := chatResp.Choices[0]
	content := ""
	if s, ok := choice.Message.Content.(string); ok {
		content = s
	}

	return &ChatCompletionResult{
		Content:   content,
		ToolCalls: choice.Message.ToolCalls,
		Timings:   chatResp.Timings,
	}, nil
}

// ChatCompletionStream sends a streaming chat completion request via POST /v1/chat/completions.
// The callback receives text chunks. Tool calls are accumulated from streamed deltas.
func (c *Client) ChatCompletionStream(ctx context.Context, messages []ChatMessage, tools []Tool, opts *CompletionRequest, callback StreamCallback) (*ChatCompletionResult, error) {
	reqBody := map[string]any{
		"messages": messages,
		"stream":   true,
	}
	if len(tools) > 0 {
		reqBody["tools"] = tools
	}
	if opts != nil {
		applyChatOpts(reqBody, opts)
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chat completion stream failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var contentBuilder strings.Builder
	var toolCalls []ToolCall
	var lastTimings *Timings

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk ChatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.Timings != nil {
			lastTimings = chunk.Timings
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta

		if delta.Content != "" {
			contentBuilder.WriteString(delta.Content)
			if err := callback(delta.Content); err != nil {
				return nil, err
			}
		}

		// Accumulate tool calls by index
		for _, tc := range delta.ToolCalls {
			for tc.Index >= len(toolCalls) {
				toolCalls = append(toolCalls, ToolCall{})
			}
			if tc.ID != "" {
				toolCalls[tc.Index].ID = tc.ID
				toolCalls[tc.Index].Type = tc.Type
				toolCalls[tc.Index].Index = tc.Index
			}
			if tc.Function.Name != "" {
				toolCalls[tc.Index].Function.Name = tc.Function.Name
			}
			toolCalls[tc.Index].Function.Arguments += tc.Function.Arguments
		}
	}

	return &ChatCompletionResult{
		Content:   contentBuilder.String(),
		ToolCalls: toolCalls,
		Timings:   lastTimings,
	}, scanner.Err()
}

// applyChatOpts maps CompletionRequest fields to /v1/chat/completions parameters
func applyChatOpts(req map[string]any, opts *CompletionRequest) {
	if opts.Temperature > 0 {
		req["temperature"] = opts.Temperature
	}
	if opts.NPredict > 0 {
		req["max_tokens"] = opts.NPredict
	}
	if opts.TopP > 0 {
		req["top_p"] = opts.TopP
	}
	if opts.TopK > 0 {
		req["top_k"] = opts.TopK
	}
	if opts.MinP > 0 {
		req["min_p"] = opts.MinP
	}
	if len(opts.Stop) > 0 {
		req["stop"] = opts.Stop
	}
	if opts.FrequencyPenalty != 0 {
		req["frequency_penalty"] = opts.FrequencyPenalty
	}
	if opts.PresencePenalty != 0 {
		req["presence_penalty"] = opts.PresencePenalty
	}
	if opts.RepeatPenalty > 0 {
		req["repeat_penalty"] = opts.RepeatPenalty
	}
	if opts.Seed > 0 {
		req["seed"] = opts.Seed
	}
	if opts.CachePrompt != nil {
		req["cache_prompt"] = *opts.CachePrompt
	}
}

// Tokenize converts text to tokens via POST /tokenize
func (c *Client) Tokenize(ctx context.Context, req *TokenizeRequest) (*TokenizeResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/tokenize", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tokenize failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result TokenizeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Detokenize converts tokens to text via POST /detokenize
func (c *Client) Detokenize(ctx context.Context, req *DetokenizeRequest) (*DetokenizeResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/detokenize", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("detokenize failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result DetokenizeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Infill sends a fill-in-the-middle request via POST /infill
func (c *Client) Infill(ctx context.Context, req *InfillRequest) (*InfillResponse, error) {
	req.Stream = false
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/infill", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("infill request failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result InfillResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SlotSave saves the KV cache state of a llama.cpp slot to a file on the server.
func (c *Client) SlotSave(ctx context.Context, slotID int, filename string) error {
	body, err := json.Marshal(map[string]string{"filename": filename})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/slots/%d?action=save", c.baseURL, slotID)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slot save failed (status %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// SlotErase clears the KV cache for a llama.cpp slot, freeing server memory.
func (c *Client) SlotErase(ctx context.Context, slotID int) error {
	url := fmt.Sprintf("%s/slots/%d?action=erase", c.baseURL, slotID)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slot erase failed (status %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// SlotRestore restores a previously saved KV cache state for a llama.cpp slot.
func (c *Client) SlotRestore(ctx context.Context, slotID int, filename string) error {
	body, err := json.Marshal(map[string]string{"filename": filename})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/slots/%d?action=restore", c.baseURL, slotID)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slot restore failed (status %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}
