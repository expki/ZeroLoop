package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/expki/ZeroLoop.git/logger"
)

// MCPServer represents a connection to an MCP server
type MCPServer struct {
	Name    string
	Command string
	Args    []string

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	mu     sync.Mutex
	nextID atomic.Int64
	tools  []MCPTool
}

// MCPTool represents a tool provided by an MCP server
type MCPTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	ServerName  string         `json:"-"`
}

// JSONRPCRequest is a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// JSONRPCResponse is a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError is a JSON-RPC 2.0 error
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewMCPServer creates a new MCP server connection
func NewMCPServer(name, command string, args []string) *MCPServer {
	return &MCPServer{
		Name:    name,
		Command: command,
		Args:    args,
	}
}

// Start starts the MCP server process and initializes the connection
func (s *MCPServer) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cmd = exec.CommandContext(ctx, s.Command, s.Args...)

	var err error
	s.stdin, err = s.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := s.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	s.stdout = bufio.NewReader(stdout)

	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("start mcp server %s: %w", s.Name, err)
	}

	// Initialize the MCP connection
	if err := s.initialize(); err != nil {
		s.cmd.Process.Kill()
		return fmt.Errorf("initialize mcp %s: %w", s.Name, err)
	}

	// List available tools
	if err := s.listTools(); err != nil {
		logger.Log.Warnw("failed to list mcp tools", "server", s.Name, "error", err)
	}

	return nil
}

// Stop shuts down the MCP server process
func (s *MCPServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stdin != nil {
		s.stdin.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
}

// Tools returns the tools provided by this server
func (s *MCPServer) Tools() []MCPTool {
	return s.tools
}

func (s *MCPServer) sendRequest(method string, params any) (*JSONRPCResponse, error) {
	id := s.nextID.Add(1)
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	// Send request with newline delimiter
	if _, err := fmt.Fprintf(s.stdin, "%s\n", data); err != nil {
		return nil, fmt.Errorf("write to mcp: %w", err)
	}

	// Read response
	line, err := s.stdout.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("read from mcp: %w", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("parse mcp response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("mcp error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return &resp, nil
}

func (s *MCPServer) initialize() error {
	_, err := s.sendRequest("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]string{
			"name":    "ZeroLoop",
			"version": "1.0.0",
		},
	})
	if err != nil {
		return err
	}

	// Send initialized notification (no ID, no response expected)
	data, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})
	fmt.Fprintf(s.stdin, "%s\n", data)

	return nil
}

func (s *MCPServer) listTools() error {
	resp, err := s.sendRequest("tools/list", nil)
	if err != nil {
		return err
	}

	var result struct {
		Tools []MCPTool `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return err
	}

	for i := range result.Tools {
		result.Tools[i].ServerName = s.Name
	}
	s.tools = result.Tools

	logger.Log.Infow("loaded mcp tools", "server", s.Name, "count", len(s.tools))
	return nil
}

// CallTool invokes a tool on the MCP server
func (s *MCPServer) CallTool(name string, args map[string]any) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resp, err := s.sendRequest("tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return string(resp.Result), nil
	}

	var texts []string
	for _, c := range result.Content {
		if c.Type == "text" {
			texts = append(texts, c.Text)
		}
	}
	return strings.Join(texts, "\n"), nil
}

// MCPManager manages multiple MCP server connections
type MCPManager struct {
	servers map[string]*MCPServer
	mu      sync.RWMutex
}

// NewMCPManager creates a new MCP manager
func NewMCPManager() *MCPManager {
	return &MCPManager{
		servers: make(map[string]*MCPServer),
	}
}

// AddServer adds and starts an MCP server
func (m *MCPManager) AddServer(ctx context.Context, name, command string, args []string) error {
	server := NewMCPServer(name, command, args)
	if err := server.Start(ctx); err != nil {
		return err
	}

	m.mu.Lock()
	m.servers[name] = server
	m.mu.Unlock()

	return nil
}

// AllTools returns all tools from all connected servers
func (m *MCPManager) AllTools() []MCPTool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var allTools []MCPTool
	for _, server := range m.servers {
		allTools = append(allTools, server.Tools()...)
	}
	return allTools
}

// CallTool finds the right server and calls the tool
func (m *MCPManager) CallTool(name string, args map[string]any) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, server := range m.servers {
		for _, tool := range server.Tools() {
			if tool.Name == name {
				return server.CallTool(name, args)
			}
		}
	}
	return "", fmt.Errorf("mcp tool not found: %s", name)
}

// StopAll shuts down all MCP servers
func (m *MCPManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, server := range m.servers {
		server.Stop()
	}
}

