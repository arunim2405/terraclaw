// Package opencode manages the OpenCode coding agent server lifecycle and
// provides a Go client for its REST API. OpenCode runs as a background
// process and exposes session management and prompting capabilities.
//
// Architecture note: This package is provider-agnostic from terraclaw's
// perspective. The actual LLM provider (Anthropic, OpenAI, Google, etc.)
// is configured inside OpenCode via its own config (opencode.json or /connect).
package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/arunim2405/terraclaw/internal/debuglog"
)

// DefaultPort is the default port for the OpenCode server.
const DefaultPort = 4096

// Server wraps a running OpenCode background process and an HTTP client
// to interact with its REST API.
type Server struct {
	cmd     *exec.Cmd
	baseURL string
	client  *http.Client
	port    int
}

// StartServer launches `opencode` as a background process and waits
// for the HTTP server to become ready.
func StartServer(ctx context.Context, port int, cwd string) (*Server, error) {
	if port == 0 {
		port = DefaultPort
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Check if an OpenCode server is already running on this port.
	existing := &Server{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 600 * time.Second},
		port:    port,
	}
	if err := existing.waitForReady(2 * time.Second); err == nil {
		debuglog.Log("[opencode] reusing existing server at %s", baseURL)
		return existing, nil
	}

	debuglog.Log("[opencode] starting server on port %d, cwd=%s", port, cwd)

	// Build the command. OpenCode is started without the TUI (headless/server mode).
	cmd := exec.CommandContext(ctx, "opencode", "serve", "--port", fmt.Sprintf("%d", port))
	cmd.Dir = cwd
	cmd.Stdout = debuglog.Writer("[opencode:stdout]")
	cmd.Stderr = debuglog.Writer("[opencode:stderr]")

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start opencode: %w", err)
	}

	debuglog.Log("[opencode] process started (pid=%d), waiting for server...", cmd.Process.Pid)

	s := &Server{
		cmd:     cmd,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 600 * time.Second},
		port:    port,
	}

	// Wait for the server to be ready (up to 30 seconds).
	if err := s.waitForReady(30 * time.Second); err != nil {
		_ = s.Stop()
		return nil, fmt.Errorf("opencode server not ready: %w", err)
	}

	debuglog.Log("[opencode] server ready at %s", baseURL)
	return s, nil
}

// ConnectToExisting connects to an already-running OpenCode server.
func ConnectToExisting(port int) *Server {
	if port == 0 {
		port = DefaultPort
	}
	return &Server{
		baseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		client:  &http.Client{Timeout: 600 * time.Second},
		port:    port,
	}
}

// Stop terminates the background OpenCode process.
func (s *Server) Stop() error {
	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}
	debuglog.Log("[opencode] stopping server (pid=%d)", s.cmd.Process.Pid)
	if err := s.cmd.Process.Signal(os.Interrupt); err != nil {
		// Fallback to kill.
		return s.cmd.Process.Kill()
	}
	// Wait briefly for graceful shutdown.
	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		return s.cmd.Process.Kill()
	}
	return nil
}

// BaseURL returns the server's base URL.
func (s *Server) BaseURL() string { return s.baseURL }

// waitForReady polls the server until it responds or timeout.
func (s *Server) waitForReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := s.client.Get(s.baseURL + "/session")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout after %s waiting for opencode server at %s", timeout, s.baseURL)
}

// ---------------------------------------------------------------------------
// REST API types
// ---------------------------------------------------------------------------

// Session represents an OpenCode session.
type Session struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// PromptPart is a content part in a prompt message.
type PromptPart struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

// PromptRequest is the body for POST /session/{id}/prompt.
type PromptRequest struct {
	Parts        []PromptPart  `json:"parts"`
	NoReply      bool          `json:"noReply,omitempty"`
	Model        *ModelRef     `json:"model,omitempty"`
	OutputFormat *OutputFormat `json:"outputFormat,omitempty"`
}

// ModelRef specifies which model to use.
type ModelRef struct {
	ProviderID string `json:"providerID"`
	ModelID    string `json:"modelID"`
}

// OutputFormat for structured output.
type OutputFormat struct {
	Type string `json:"type"` // "text"
}

// MessageInfo is the info portion of a message response.
type MessageInfo struct {
	ID   string `json:"id"`
	Role string `json:"role"` // "assistant"
}

// MessagePart is a part of an assistant message.
type MessagePart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ToolName string          `json:"toolName,omitempty"` // for tool-use/tool-invocation parts
	State    json.RawMessage `json:"state,omitempty"`    // can be object or string
	Input    json.RawMessage `json:"input,omitempty"`    // tool input arguments
	Output   json.RawMessage `json:"output,omitempty"`   // tool-result output
}

// IsThinking returns true if this part is a thinking/reasoning block.
func (p MessagePart) IsThinking() bool {
	return p.Type == "thinking" || p.Type == "reasoning"
}

// IsToolUse returns true if this part is a tool invocation.
func (p MessagePart) IsToolUse() bool {
	return p.Type == "tool-invocation" || p.Type == "tool-use"
}

// IsToolResult returns true if this part is a tool result.
func (p MessagePart) IsToolResult() bool {
	return p.Type == "tool-result"
}

// IsText returns true if this part is regular text output.
func (p MessagePart) IsText() bool {
	return p.Type == "text"
}

// OutputString returns the tool output as a string.
func (p MessagePart) OutputString() string {
	if len(p.Output) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(p.Output, &s) == nil {
		return s
	}
	return string(p.Output)
}

// StateString returns a human-readable state from the raw JSON state field.
func (p MessagePart) StateString() string {
	if len(p.State) == 0 {
		return ""
	}
	// Try as a plain string first.
	var s string
	if json.Unmarshal(p.State, &s) == nil {
		return s
	}
	// Try as an object with a "status" field.
	var obj map[string]interface{}
	if json.Unmarshal(p.State, &obj) == nil {
		if status, ok := obj["status"].(string); ok {
			return status
		}
	}
	return string(p.State)
}

// AssistantMessage is the response from a prompt call.
type AssistantMessage struct {
	Info  MessageInfo   `json:"info"`
	Parts []MessagePart `json:"parts"`
}

// SessionMessage represents a message entry returned by the list messages API.
type SessionMessage struct {
	Info  MessageInfo   `json:"info"`
	Parts []MessagePart `json:"parts"`
}

// PromptResult carries the result of an async prompt call.
type PromptResult struct {
	Response string
	Err      error
}

// ---------------------------------------------------------------------------
// API methods
// ---------------------------------------------------------------------------

// CreateSession creates a new OpenCode session and returns its ID.
func (s *Server) CreateSession(title string) (string, error) {
	body := map[string]string{"title": title}
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	resp, err := s.client.Post(s.baseURL+"/session", "application/json", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create session: status %d: %s", resp.StatusCode, string(respBody))
	}

	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return "", fmt.Errorf("decode session: %w", err)
	}

	debuglog.Log("[opencode] created session id=%s title=%q", session.ID, session.Title)
	return session.ID, nil
}

// InjectSystemPrompt sends a system-level instruction to the session without
// triggering an AI response. This is used to set up the HashiCorp Terraform
// style guide before sending the actual resource prompt.
func (s *Server) InjectSystemPrompt(sessionID, prompt string) error {
	req := PromptRequest{
		NoReply: true,
		Parts: []PromptPart{
			{Type: "text", Text: prompt},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/session/%s/message", s.baseURL, sessionID)
	resp, err := s.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("inject system prompt: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("inject system prompt: status %d: %s", resp.StatusCode, string(respBody))
	}

	debuglog.Log("[opencode] injected system prompt into session %s (%d bytes)", sessionID, len(prompt))
	return nil
}

// Prompt sends a user message and waits for the assistant's response.
func (s *Server) Prompt(sessionID, prompt string) (string, error) {
	req := PromptRequest{
		Parts: []PromptPart{
			{Type: "text", Text: prompt},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/session/%s/message", s.baseURL, sessionID)
	debuglog.Log("[opencode] sending prompt to session %s (%d bytes)", sessionID, len(prompt))

	resp, err := s.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("send prompt: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("prompt: status %d: %s", resp.StatusCode, string(respBody))
	}

	var msg AssistantMessage
	bodyAsStr := string(respBody)

	if err := json.Unmarshal(respBody, &msg); err != nil {
		// Sometimes OpenCode API returns plain text on errors (e.g. rate limit, decode error) despite a 200 HTTP code.
		return "", fmt.Errorf("opencode API error: %s", bodyAsStr)
	}

	// Extract all text parts from the response.
	var result string
	for _, part := range msg.Parts {
		if part.Type == "text" {
			result += part.Text
		}
	}

	debuglog.Log("[opencode] received response from session %s (%d bytes)", sessionID, len(result))
	return result, nil
}

// ListMessages fetches all messages in a session. This is used to poll
// for progress during code generation.
func (s *Server) ListMessages(sessionID string) ([]SessionMessage, error) {
	url := fmt.Sprintf("%s/session/%s/message", s.baseURL, sessionID)

	// Use a short-timeout client for polling — don't tie up the long-timeout client.
	pollClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := pollClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list messages: status %d: %s", resp.StatusCode, string(respBody))
	}

	var messages []SessionMessage
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, fmt.Errorf("decode messages: %w", err)
	}
	return messages, nil
}

// ---------------------------------------------------------------------------
// Message tracking for deduplication across polls
// ---------------------------------------------------------------------------

// MessageTracker tracks which message parts have already been processed
// so that repeated polling does not re-display the same content.
type MessageTracker struct {
	// seenParts maps "messageID:partIndex" to true.
	seenParts map[string]bool
}

// NewMessageTracker creates a new tracker.
func NewMessageTracker() *MessageTracker {
	return &MessageTracker{seenParts: make(map[string]bool)}
}

// NewParts returns only the message parts that haven't been seen before,
// across all messages. It marks returned parts as seen. Each returned entry
// includes the originating message role for context.
func (t *MessageTracker) NewParts(messages []SessionMessage) []TrackedPart {
	var result []TrackedPart
	for _, msg := range messages {
		for i, part := range msg.Parts {
			key := fmt.Sprintf("%s:%d", msg.Info.ID, i)
			if t.seenParts[key] {
				continue
			}
			t.seenParts[key] = true
			result = append(result, TrackedPart{
				Role: msg.Info.Role,
				Part: part,
			})
		}
	}
	return result
}

// TrackedPart pairs a message part with its originating role.
type TrackedPart struct {
	Role string
	Part MessagePart
}

// PromptAsync sends a prompt in a background goroutine and returns
// a channel that will receive the result when the prompt completes.
// This allows the caller to poll for progress while waiting.
func (s *Server) PromptAsync(sessionID, prompt string) <-chan PromptResult {
	ch := make(chan PromptResult, 1)
	go func() {
		response, err := s.Prompt(sessionID, prompt)
		ch <- PromptResult{Response: response, Err: err}
	}()
	return ch
}
