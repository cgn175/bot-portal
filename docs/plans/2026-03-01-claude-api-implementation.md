# Claude API Support - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add native Claude API support to Bot Portal with dual-format chat endpoint

**Architecture:** Implement `/api/claude` endpoint for direct Claude API compatibility and enhance `/api/chat/completions` to auto-detect Claude models (via "claude-" prefix) and transform between OpenAI and Claude formats bidirectionally.

**Tech Stack:** Go 1.21+, net/http, encoding/json, bufio (SSE parsing)

---

## Task 1: Claude Data Types

**Files:**
- Create: `internal/api/claude.go`
- Test: `internal/api/claude_test.go`

**Step 1: Write the failing test**

Create `internal/api/claude_test.go`:

```go
package api

import (
	"encoding/json"
	"testing"
)

func TestClaudeRequest_Marshal(t *testing.T) {
	req := ClaudeRequest{
		Model:     "claude-opus-4-6",
		MaxTokens: 1024,
		Messages: []ClaudeMessage{
			{Role: "user", Content: "Hello"},
		},
		System: "You are helpful",
		Stream: true,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled ClaudeRequest
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.Model != req.Model {
		t.Errorf("model mismatch: got %s, want %s", unmarshaled.Model, req.Model)
	}
	if unmarshaled.System != req.System {
		t.Errorf("system mismatch: got %s, want %s", unmarshaled.System, req.System)
	}
}

func TestClaudeResponse_Unmarshal(t *testing.T) {
	jsonData := `{
		"id": "msg_123",
		"type": "message",
		"role": "assistant",
		"content": [{"type": "text", "text": "Hello!"}],
		"model": "claude-opus-4-6",
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 10, "output_tokens": 5}
	}`

	var resp ClaudeResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.ID != "msg_123" {
		t.Errorf("id mismatch: got %s, want msg_123", resp.ID)
	}
	if len(resp.Content) != 1 {
		t.Errorf("content length: got %d, want 1", len(resp.Content))
	}
	if resp.Content[0].Text != "Hello!" {
		t.Errorf("content text: got %s, want Hello!", resp.Content[0].Text)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/api/ -run TestClaudeRequest_Marshal`
Expected: FAIL with "undefined: ClaudeRequest"

**Step 3: Write minimal implementation**

Create `internal/api/claude.go`:

```go
package api

// ClaudeRequest represents a Claude API request
type ClaudeRequest struct {
	Model         string          `json:"model"`
	Messages      []ClaudeMessage `json:"messages"`
	System        string          `json:"system,omitempty"`
	MaxTokens     int             `json:"max_tokens"`
	Temperature   float64         `json:"temperature,omitempty"`
	TopP          float64         `json:"top_p,omitempty"`
	TopK          int             `json:"top_k,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
}

// ClaudeMessage represents a message in Claude format
type ClaudeMessage struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// ClaudeResponse represents a Claude API response
type ClaudeResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"` // "message"
	Role         string         `json:"role"` // "assistant"
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence *string        `json:"stop_sequence,omitempty"`
	Usage        ClaudeUsage    `json:"usage"`
}

// ContentBlock represents a content block in Claude response
type ContentBlock struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

// ClaudeUsage represents token usage in Claude response
type ClaudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ClaudeStreamEvent represents a streaming event from Claude
type ClaudeStreamEvent struct {
	Type  string        `json:"type"` // "message_start", "content_block_delta", etc.
	Index int           `json:"index,omitempty"`
	Delta *ContentDelta `json:"delta,omitempty"`
}

// ContentDelta represents a delta in streaming response
type ContentDelta struct {
	Type string `json:"type"` // "text_delta"
	Text string `json:"text"`
}

// ClaudeError represents a Claude API error
type ClaudeError struct {
	Type  string           `json:"type"` // "error"
	Error ClaudeErrorDetail `json:"error"`
}

// ClaudeErrorDetail represents error details
type ClaudeErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/api/ -run TestClaudeRequest_Marshal`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/claude.go internal/api/claude_test.go
git commit -m "feat: add Claude API data types

- Add ClaudeRequest, ClaudeResponse types
- Add ClaudeMessage, ContentBlock types
- Add streaming types (ClaudeStreamEvent, ContentDelta)
- Add error types (ClaudeError)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 2: Model Detection

**Files:**
- Modify: `internal/api/claude.go`
- Test: `internal/api/claude_test.go`

**Step 1: Write the failing test**

Add to `internal/api/claude_test.go`:

```go
func TestIsClaudeModel(t *testing.T) {
	tests := []struct {
		modelName string
		expected  bool
	}{
		{"claude-opus-4-6", true},
		{"claude-sonnet-4", true},
		{"claude-3-opus-20240229", true},
		{"CLAUDE-OPUS-4-6", true}, // Case insensitive
		{"gpt-4", false},
		{"gpt-4o", false},
		{"text-davinci-003", false},
		{"gemini-pro", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.modelName, func(t *testing.T) {
			result := isClaudeModel(tt.modelName)
			if result != tt.expected {
				t.Errorf("isClaudeModel(%q) = %v, want %v", tt.modelName, result, tt.expected)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/api/ -run TestIsClaudeModel`
Expected: FAIL with "undefined: isClaudeModel"

**Step 3: Write minimal implementation**

Add to `internal/api/claude.go`:

```go
import "strings"

// isClaudeModel returns true if the model name indicates a Claude model
func isClaudeModel(modelName string) bool {
	return strings.HasPrefix(strings.ToLower(modelName), "claude-")
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/api/ -run TestIsClaudeModel`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/claude.go internal/api/claude_test.go
git commit -m "feat: add Claude model detection

- Implement isClaudeModel() function
- Check for 'claude-' prefix (case insensitive)
- Add comprehensive test cases

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 3: OpenAI to Claude Transformation

**Files:**
- Modify: `internal/api/claude.go`
- Test: `internal/api/claude_test.go`

**Step 1: Write the failing test**

Add to `internal/api/claude_test.go`:

```go
func TestTransformToClaudeFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    ChatRequest
		expected ClaudeRequest
	}{
		{
			name: "extract system message",
			input: ChatRequest{
				Model: "claude-opus-4-6",
				Messages: []ChatMessage{
					{Role: "system", Content: "You are helpful"},
					{Role: "user", Content: "Hello"},
				},
				MaxTokens:   1024,
				Temperature: 0.7,
				Stream:      true,
			},
			expected: ClaudeRequest{
				Model:       "claude-opus-4-6",
				System:      "You are helpful",
				Messages:    []ClaudeMessage{{Role: "user", Content: "Hello"}},
				MaxTokens:   1024,
				Temperature: 0.7,
				Stream:      true,
			},
		},
		{
			name: "no system message",
			input: ChatRequest{
				Model: "claude-sonnet-4",
				Messages: []ChatMessage{
					{Role: "user", Content: "Hi"},
					{Role: "assistant", Content: "Hello!"},
					{Role: "user", Content: "How are you?"},
				},
				MaxTokens: 512,
			},
			expected: ClaudeRequest{
				Model:     "claude-sonnet-4",
				System:    "",
				Messages: []ClaudeMessage{
					{Role: "user", Content: "Hi"},
					{Role: "assistant", Content: "Hello!"},
					{Role: "user", Content: "How are you?"},
				},
				MaxTokens: 512,
			},
		},
		{
			name: "multiple system messages - use first",
			input: ChatRequest{
				Model: "claude-opus-4-6",
				Messages: []ChatMessage{
					{Role: "system", Content: "First system"},
					{Role: "user", Content: "Hello"},
					{Role: "system", Content: "Second system"},
				},
				MaxTokens: 100,
			},
			expected: ClaudeRequest{
				Model:     "claude-opus-4-6",
				System:    "First system",
				Messages:  []ClaudeMessage{{Role: "user", Content: "Hello"}},
				MaxTokens: 100,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformToClaudeFormat(tt.input)

			if result.Model != tt.expected.Model {
				t.Errorf("model: got %s, want %s", result.Model, tt.expected.Model)
			}
			if result.System != tt.expected.System {
				t.Errorf("system: got %q, want %q", result.System, tt.expected.System)
			}
			if len(result.Messages) != len(tt.expected.Messages) {
				t.Errorf("messages length: got %d, want %d", len(result.Messages), len(tt.expected.Messages))
			}
			for i := range result.Messages {
				if result.Messages[i].Role != tt.expected.Messages[i].Role {
					t.Errorf("message[%d] role: got %s, want %s", i, result.Messages[i].Role, tt.expected.Messages[i].Role)
				}
				if result.Messages[i].Content != tt.expected.Messages[i].Content {
					t.Errorf("message[%d] content: got %s, want %s", i, result.Messages[i].Content, tt.expected.Messages[i].Content)
				}
			}
			if result.MaxTokens != tt.expected.MaxTokens {
				t.Errorf("max_tokens: got %d, want %d", result.MaxTokens, tt.expected.MaxTokens)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/api/ -run TestTransformToClaudeFormat`
Expected: FAIL with "undefined: transformToClaudeFormat"

**Step 3: Write minimal implementation**

Add to `internal/api/claude.go`:

```go
// transformToClaudeFormat converts OpenAI ChatRequest to Claude format
func transformToClaudeFormat(openAIReq ChatRequest) ClaudeRequest {
	claudeReq := ClaudeRequest{
		Model:       openAIReq.Model,
		MaxTokens:   openAIReq.MaxTokens,
		Messages:    []ClaudeMessage{},
		Stream:      openAIReq.Stream,
		Temperature: openAIReq.Temperature,
		TopP:        openAIReq.TopP,
	}

	// Extract system message and filter out from messages array
	for _, msg := range openAIReq.Messages {
		if msg.Role == "system" {
			// Use first system message only
			if claudeReq.System == "" {
				claudeReq.System = msg.Content
			}
		} else {
			// Keep user and assistant messages
			claudeReq.Messages = append(claudeReq.Messages, ClaudeMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	return claudeReq
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/api/ -run TestTransformToClaudeFormat`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/claude.go internal/api/claude_test.go
git commit -m "feat: add OpenAI to Claude transformation

- Implement transformToClaudeFormat()
- Extract system messages to top-level system param
- Filter system messages from messages array
- Handle multiple system messages (use first)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 4: Claude to OpenAI Response Transformation

**Files:**
- Modify: `internal/api/claude.go`
- Test: `internal/api/claude_test.go`

**Step 1: Write the failing test**

Add to `internal/api/claude_test.go`:

```go
func TestTransformClaudeResponseToOpenAI(t *testing.T) {
	tests := []struct {
		name     string
		input    ClaudeResponse
		expected OpenAIResponse
	}{
		{
			name: "basic response",
			input: ClaudeResponse{
				ID:   "msg_123",
				Type: "message",
				Role: "assistant",
				Content: []ContentBlock{
					{Type: "text", Text: "Hello! How can I help?"},
				},
				Model:      "claude-opus-4-6",
				StopReason: "end_turn",
				Usage: ClaudeUsage{
					InputTokens:  10,
					OutputTokens: 20,
				},
			},
			expected: OpenAIResponse{
				ID:      "msg_123",
				Object:  "chat.completion",
				Model:   "claude-opus-4-6",
				Choices: []Choice{
					{
						Index: 0,
						Message: Message{
							Role:    "assistant",
							Content: "Hello! How can I help?",
						},
						FinishReason: "stop",
					},
				},
				Usage: Usage{
					PromptTokens:     10,
					CompletionTokens: 20,
					TotalTokens:      30,
				},
			},
		},
		{
			name: "max_tokens stop reason",
			input: ClaudeResponse{
				ID:   "msg_456",
				Type: "message",
				Role: "assistant",
				Content: []ContentBlock{
					{Type: "text", Text: "Response text"},
				},
				Model:      "claude-sonnet-4",
				StopReason: "max_tokens",
				Usage: ClaudeUsage{
					InputTokens:  5,
					OutputTokens: 100,
				},
			},
			expected: OpenAIResponse{
				ID:      "msg_456",
				Object:  "chat.completion",
				Model:   "claude-sonnet-4",
				Choices: []Choice{
					{
						Index: 0,
						Message: Message{
							Role:    "assistant",
							Content: "Response text",
						},
						FinishReason: "length",
					},
				},
				Usage: Usage{
					PromptTokens:     5,
					CompletionTokens: 100,
					TotalTokens:      105,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformClaudeResponseToOpenAI(tt.input)

			if result.ID != tt.expected.ID {
				t.Errorf("id: got %s, want %s", result.ID, tt.expected.ID)
			}
			if result.Model != tt.expected.Model {
				t.Errorf("model: got %s, want %s", result.Model, tt.expected.Model)
			}
			if len(result.Choices) != len(tt.expected.Choices) {
				t.Fatalf("choices length: got %d, want %d", len(result.Choices), len(tt.expected.Choices))
			}
			if result.Choices[0].Message.Content != tt.expected.Choices[0].Message.Content {
				t.Errorf("content: got %s, want %s", result.Choices[0].Message.Content, tt.expected.Choices[0].Message.Content)
			}
			if result.Choices[0].FinishReason != tt.expected.Choices[0].FinishReason {
				t.Errorf("finish_reason: got %s, want %s", result.Choices[0].FinishReason, tt.expected.Choices[0].FinishReason)
			}
			if result.Usage.TotalTokens != tt.expected.Usage.TotalTokens {
				t.Errorf("total_tokens: got %d, want %d", result.Usage.TotalTokens, tt.expected.Usage.TotalTokens)
			}
		})
	}
}

func TestExtractTextContent(t *testing.T) {
	tests := []struct {
		name     string
		input    []ContentBlock
		expected string
	}{
		{
			name:     "single text block",
			input:    []ContentBlock{{Type: "text", Text: "Hello"}},
			expected: "Hello",
		},
		{
			name:     "multiple blocks - first text",
			input:    []ContentBlock{{Type: "text", Text: "First"}, {Type: "text", Text: "Second"}},
			expected: "First",
		},
		{
			name:     "empty blocks",
			input:    []ContentBlock{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTextContent(tt.input)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMapStopReason(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"end_turn", "stop"},
		{"max_tokens", "length"},
		{"stop_sequence", "stop"},
		{"unknown", "stop"},
		{"", "stop"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapStopReason(tt.input)
			if result != tt.expected {
				t.Errorf("mapStopReason(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/api/ -run TestTransformClaudeResponseToOpenAI`
Expected: FAIL with "undefined: transformClaudeResponseToOpenAI"

**Step 3: Write minimal implementation**

Add to `internal/api/claude.go`:

```go
import "time"

// transformClaudeResponseToOpenAI converts Claude response to OpenAI format
func transformClaudeResponseToOpenAI(claudeResp ClaudeResponse) OpenAIResponse {
	return OpenAIResponse{
		ID:      claudeResp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   claudeResp.Model,
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: extractTextContent(claudeResp.Content),
				},
				FinishReason: mapStopReason(claudeResp.StopReason),
			},
		},
		Usage: Usage{
			PromptTokens:     claudeResp.Usage.InputTokens,
			CompletionTokens: claudeResp.Usage.OutputTokens,
			TotalTokens:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		},
	}
}

// extractTextContent extracts text from content blocks
func extractTextContent(content []ContentBlock) string {
	for _, block := range content {
		if block.Type == "text" {
			return block.Text
		}
	}
	return ""
}

// mapStopReason maps Claude stop reason to OpenAI finish reason
func mapStopReason(claudeReason string) string {
	switch claudeReason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	default:
		return "stop"
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/api/ -run TestTransformClaudeResponseToOpenAI`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/claude.go internal/api/claude_test.go
git commit -m "feat: add Claude to OpenAI response transformation

- Implement transformClaudeResponseToOpenAI()
- Extract text from content blocks
- Map stop_reason to finish_reason
- Transform usage tokens (input_tokens → prompt_tokens)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 5: Claude Endpoint Handler

**Files:**
- Modify: `internal/api/claude.go`
- Modify: `internal/api/router.go`

**Step 1: Write the failing test**

Add to `internal/api/claude_test.go`:

```go
import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleClaudeMessages(t *testing.T) {
	// Create test router with in-memory database
	db, _ := store.NewSQLiteStore(":memory:")
	router := &Router{
		modelStore: db,
		// ... other dependencies
	}

	// Create test Claude request
	claudeReq := ClaudeRequest{
		Model:     "claude-opus-4-6",
		MaxTokens: 100,
		Messages: []ClaudeMessage{
			{Role: "user", Content: "Hello"},
		},
		Stream: false,
	}

	body, _ := json.Marshal(claudeReq)
	req := httptest.NewRequest("POST", "/api/claude", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.handleClaudeMessages(w, req)

	// For now, expect 501 Not Implemented (since we haven't implemented proxying yet)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("status code: got %d, want %d", w.Code, http.StatusNotImplemented)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/api/ -run TestHandleClaudeMessages`
Expected: FAIL with "undefined: Router.handleClaudeMessages"

**Step 3: Write minimal implementation**

Add to `internal/api/claude.go`:

```go
import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// handleClaudeMessages handles POST /api/claude
func (r *Router) handleClaudeMessages(w http.ResponseWriter, req *http.Request) {
	// Log request details (user requirement: "with log detail about the request of claude code")
	log.Printf("[Claude API] Request: method=%s, path=%s, remote=%s",
		req.Method, req.URL.Path, req.RemoteAddr)

	// Parse Claude request
	var claudeReq ClaudeRequest
	if err := json.NewDecoder(req.Body).Decode(&claudeReq); err != nil {
		log.Printf("[Claude API] Failed to parse request: %v", err)
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Log request details
	log.Printf("[Claude API] Request details: model=%s, messages=%d, stream=%v, max_tokens=%d",
		claudeReq.Model, len(claudeReq.Messages), claudeReq.Stream, claudeReq.MaxTokens)

	if claudeReq.System != "" {
		log.Printf("[Claude API] System message: %s", claudeReq.System)
	}

	// TODO: Implement provider proxy
	// For now, return 501 Not Implemented
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{
		"error": "Provider proxy not yet implemented",
	})
}
```

Add to `internal/api/router.go` in the `Run()` method:

```go
// Claude API endpoint
mux.HandleFunc("POST /api/claude", r.handleClaudeMessages)
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/api/ -run TestHandleClaudeMessages`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/claude.go internal/api/claude_test.go internal/api/router.go
git commit -m "feat: add /api/claude endpoint handler

- Implement handleClaudeMessages() handler
- Parse Claude request format
- Add detailed logging per user requirement
- Register route in router
- Return 501 for now (proxy not implemented)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 6: Adaptive Chat Endpoint

**Files:**
- Modify: `internal/api/chat.go`
- Test: `internal/api/chat_test.go`

**Step 1: Write the failing test**

Add to `internal/api/chat_test.go`:

```go
func TestHandleChatCompletions_ClaudeModel(t *testing.T) {
	// Create test router with in-memory database
	db, _ := store.NewSQLiteStore(":memory:")

	// Create a Claude model in database
	model := &models.Model{
		ID:              "claude-test",
		Name:            "Claude Test",
		Provider:        "anthropic",
		ModelIdentifier: "claude-opus-4-6",
		EndpointURL:     "https://api.anthropic.com/v1",
	}
	db.CreateModel(model)

	router := &Router{modelStore: db}

	// Send OpenAI-format request with Claude model
	chatReq := ChatRequest{
		Model: "claude-test",
		Messages: []ChatMessage{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello"},
		},
		MaxTokens: 100,
		Stream:    false,
	}

	body, _ := json.Marshal(chatReq)
	req := httptest.NewRequest("POST", "/api/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.handleChatCompletions(w, req)

	// For now, expect 501 (transformation logic not yet implemented)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("status code: got %d, want %d", w.Code, http.StatusNotImplemented)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/api/ -run TestHandleChatCompletions_ClaudeModel`
Expected: FAIL (test doesn't exist yet or passes incorrectly)

**Step 3: Write minimal implementation**

Modify `internal/api/chat.go` in the `handleChatCompletions` method:

```go
func (r *Router) handleChatCompletions(w http.ResponseWriter, req *http.Request) {
	var chatReq ChatRequest
	if err := json.NewDecoder(req.Body).Decode(&chatReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get model from database
	model, err := r.modelStore.GetModelByID(chatReq.Model)
	if err != nil {
		http.Error(w, fmt.Sprintf("Model not found: %s", chatReq.Model), http.StatusNotFound)
		return
	}

	// Check if this is a Claude model
	if isClaudeModel(model.ModelIdentifier) {
		log.Printf("[Adaptive Chat] Detected Claude model: %s", model.ModelIdentifier)

		// Transform OpenAI → Claude
		claudeReq := transformToClaudeFormat(chatReq)

		log.Printf("[Format Transform] OpenAI → Claude: system=%v, messages=%d",
			len(claudeReq.System) > 0, len(claudeReq.Messages))

		// TODO: Proxy as Claude request
		w.WriteHeader(http.StatusNotImplemented)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Claude model proxy not yet implemented",
		})
		return
	}

	// Keep existing OpenAI logic
	log.Printf("[Adaptive Chat] Using OpenAI format for model: %s", model.ModelIdentifier)
	// ... existing code ...
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/api/ -run TestHandleChatCompletions_ClaudeModel`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/chat.go internal/api/chat_test.go
git commit -m "feat: add adaptive format detection to chat endpoint

- Detect Claude models in /api/chat/completions
- Transform OpenAI → Claude format automatically
- Add logging for format transformation
- Return 501 for now (proxy not implemented)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 7: Streaming Transformation

**Files:**
- Modify: `internal/api/claude.go`
- Test: `internal/api/claude_test.go`

**Step 1: Write the failing test**

Add to `internal/api/claude_test.go`:

```go
import (
	"bufio"
	"strings"
)

func TestTransformClaudeStreamToOpenAI(t *testing.T) {
	// Simulate Claude SSE stream
	claudeStream := `event: message_start
data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","content":[],"model":"claude-opus-4-6"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}

event: message_stop
data: {"type":"message_stop"}
`

	reader := strings.NewReader(claudeStream)
	w := httptest.NewRecorder()

	transformClaudeStreamToOpenAI(reader, w)

	body := w.Body.String()

	// Should contain OpenAI-format chunks
	if !strings.Contains(body, `"object":"chat.completion.chunk"`) {
		t.Error("missing OpenAI chunk format")
	}
	if !strings.Contains(body, `"delta":{"content":"Hello"}`) {
		t.Error("missing first delta")
	}
	if !strings.Contains(body, `"delta":{"content":" world"}`) {
		t.Error("missing second delta")
	}
	if !strings.Contains(body, "data: [DONE]") {
		t.Error("missing [DONE] marker")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/api/ -run TestTransformClaudeStreamToOpenAI`
Expected: FAIL with "undefined: transformClaudeStreamToOpenAI"

**Step 3: Write minimal implementation**

Add to `internal/api/claude.go`:

```go
import (
	"bufio"
	"io"
	"strings"
)

// transformClaudeStreamToOpenAI transforms Claude SSE stream to OpenAI format
func transformClaudeStreamToOpenAI(claudeStream io.Reader, w http.ResponseWriter) error {
	scanner := bufio.NewScanner(claudeStream)
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for scanner.Scan() {
		line := scanner.Text()

		// Skip event lines
		if strings.HasPrefix(line, "event:") {
			continue
		}

		// Process data lines
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data: ")

			var claudeEvent ClaudeStreamEvent
			if err := json.Unmarshal([]byte(data), &claudeEvent); err != nil {
				continue
			}

			// Only transform content_block_delta events
			if claudeEvent.Type == "content_block_delta" && claudeEvent.Delta != nil {
				openAIChunk := OpenAIStreamChunk{
					ID:      "chatcmpl-" + fmt.Sprintf("%d", time.Now().UnixNano()),
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   "claude", // Will be filled with actual model
					Choices: []StreamChoice{
						{
							Index: 0,
							Delta: Delta{
								Content: claudeEvent.Delta.Text,
							},
						},
					},
				}

				chunkJSON, _ := json.Marshal(openAIChunk)
				fmt.Fprintf(w, "data: %s\n\n", chunkJSON)
				flusher.Flush()
			}
		}
	}

	// Send [DONE]
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	return scanner.Err()
}

// OpenAIStreamChunk represents a streaming chunk in OpenAI format
type OpenAIStreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
}

// StreamChoice represents a choice in a streaming response
type StreamChoice struct {
	Index int   `json:"index"`
	Delta Delta `json:"delta"`
}

// Delta represents a delta in a streaming response
type Delta struct {
	Content string `json:"content,omitempty"`
	Role    string `json:"role,omitempty"`
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/api/ -run TestTransformClaudeStreamToOpenAI`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/claude.go internal/api/claude_test.go
git commit -m "feat: add Claude SSE to OpenAI SSE transformation

- Implement transformClaudeStreamToOpenAI()
- Parse Claude SSE events (content_block_delta)
- Transform to OpenAI streaming chunks
- Add [DONE] marker at end

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 8: Integration Tests

**Files:**
- Create: `test/claude_integration_test.go`

**Step 1: Write the failing test**

Create `test/claude_integration_test.go`:

```go
package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"bot-portal/internal/api"
	"bot-portal/internal/models"
	"bot-portal/internal/store"
)

func TestClaudeEndpoint_Integration(t *testing.T) {
	// Setup in-memory database
	db, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create Claude model
	model := &models.Model{
		ID:              "claude-test",
		Name:            "Claude Test",
		Provider:        "anthropic",
		ModelIdentifier: "claude-opus-4-6",
		EndpointURL:     "https://api.anthropic.com/v1",
	}
	if err := db.CreateModel(model); err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Create router
	router := api.NewRouter(db, nil, nil)

	// Test Claude endpoint
	claudeReq := api.ClaudeRequest{
		Model:     "claude-opus-4-6",
		MaxTokens: 100,
		Messages: []api.ClaudeMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	body, _ := json.Marshal(claudeReq)
	req := httptest.NewRequest("POST", "/api/claude", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 501 for now (proxy not implemented)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("status code: got %d, want %d", w.Code, http.StatusNotImplemented)
	}
}

func TestAdaptiveChatWithClaudeModel_Integration(t *testing.T) {
	// Setup in-memory database
	db, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create Claude model
	model := &models.Model{
		ID:              "claude-test",
		Name:            "Claude Test",
		Provider:        "anthropic",
		ModelIdentifier: "claude-opus-4-6",
		EndpointURL:     "https://api.anthropic.com/v1",
	}
	if err := db.CreateModel(model); err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Create router
	router := api.NewRouter(db, nil, nil)

	// Test adaptive chat endpoint with Claude model
	chatReq := api.ChatRequest{
		Model: "claude-test",
		Messages: []api.ChatMessage{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello"},
		},
		MaxTokens: 100,
	}

	body, _ := json.Marshal(chatReq)
	req := httptest.NewRequest("POST", "/api/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should detect Claude model and attempt transformation
	// For now, expect 501 (proxy not implemented)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("status code: got %d, want %d", w.Code, http.StatusNotImplemented)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./test/ -run TestClaudeEndpoint_Integration`
Expected: FAIL (may pass if code works correctly, or fail if router setup is wrong)

**Step 3: Fix any issues**

Ensure router correctly registers the `/api/claude` endpoint and adaptive logic works.

**Step 4: Run test to verify it passes**

Run: `go test -v ./test/ -run TestClaudeEndpoint_Integration`
Expected: PASS

**Step 5: Commit**

```bash
git add test/claude_integration_test.go
git commit -m "test: add Claude API integration tests

- Test /api/claude endpoint
- Test adaptive /api/chat/completions with Claude model
- Verify format detection and transformation

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 9: Documentation

**Files:**
- Modify: `CLAUDE.md`
- Create: `docs/claude-api.md`

**Step 1: Write documentation**

Create `docs/claude-api.md`:

```markdown
# Claude API Support

Bot Portal supports both OpenAI and Claude API formats with automatic detection and transformation.

## Endpoints

### 1. Native Claude Endpoint

**Endpoint:** `POST /api/claude`

**Purpose:** Direct Claude API compatibility for Claude Code CLI and other Anthropic API clients.

**Request Format:**
\`\`\`json
{
  "model": "claude-opus-4-6",
  "max_tokens": 1024,
  "messages": [
    {"role": "user", "content": "Hello"}
  ],
  "system": "You are a helpful assistant",
  "stream": true
}
\`\`\`

**Response Format (Non-Streaming):**
\`\`\`json
{
  "id": "msg_123",
  "type": "message",
  "role": "assistant",
  "content": [
    {"type": "text", "text": "Hello! How can I help?"}
  ],
  "model": "claude-opus-4-6",
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 10,
    "output_tokens": 20
  }
}
\`\`\`

### 2. Adaptive Chat Endpoint

**Endpoint:** `POST /api/chat/completions`

**Purpose:** Auto-detects model format (OpenAI vs Claude) and transforms accordingly.

**Model Detection:** Checks if model name starts with `claude-` prefix.

**Example with Claude Model:**

Send OpenAI-format request:
\`\`\`json
{
  "model": "claude-opus-4-6",
  "messages": [
    {"role": "system", "content": "You are helpful"},
    {"role": "user", "content": "Hello"}
  ]
}
\`\`\`

Bot Portal automatically:
1. Detects Claude model
2. Transforms to Claude format (extracts system message)
3. Proxies to provider
4. Transforms response back to OpenAI format

## Using with Claude Code CLI

Configure Claude Code CLI to use Bot Portal:

\`\`\`bash
export ANTHROPIC_BASE_URL=http://localhost:8080/api/claude
export ANTHROPIC_API_KEY=dummy  # Not used, but CLI requires it

claude-code "List files in current directory"
\`\`\`

Bot Portal will:
- Use models from your database
- Apply provider authentication from auth configs
- Log detailed request information

## Format Transformation

### OpenAI → Claude

- Extracts `system` messages from messages array to top-level `system` parameter
- Keeps `user` and `assistant` messages in messages array
- Maps parameters: `max_tokens`, `temperature`, `top_p`

### Claude → OpenAI

- Extracts text from `content` blocks
- Maps `stop_reason` to `finish_reason`:
  - `end_turn` → `stop`
  - `max_tokens` → `length`
  - `stop_sequence` → `stop`
- Transforms usage: `input_tokens` → `prompt_tokens`, `output_tokens` → `completion_tokens`

## Logging

Detailed logs for debugging Claude Code CLI requests:

\`\`\`
[Claude API] Request: method=POST, path=/api/claude, remote=127.0.0.1:12345
[Claude API] Request details: model=claude-opus-4-6, messages=1, stream=true, max_tokens=1024
[Format Transform] OpenAI → Claude: system=true, messages=1
\`\`\`
```

Modify `CLAUDE.md` to add:

```markdown
## Claude API Support

Bot Portal natively supports both OpenAI and Claude API formats.

**Endpoints:**
- `/api/claude` - Native Claude API format (for Claude Code CLI)
- `/api/chat/completions` - Adaptive format (auto-detects Claude models)

**Model Detection:** Any model name starting with `claude-` is automatically detected and transformed.

See `docs/claude-api.md` for full details.
```

**Step 2: Commit**

```bash
git add CLAUDE.md docs/claude-api.md
git commit -m "docs: add Claude API support documentation

- Document /api/claude endpoint
- Document adaptive chat endpoint
- Add Claude Code CLI usage guide
- Explain format transformations

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Success Criteria

✅ `/api/claude` endpoint accepts Claude-format requests
✅ `/api/chat/completions` auto-detects Claude models via prefix
✅ Format transformation (OpenAI ↔ Claude) works correctly
✅ System messages extracted/inserted properly
✅ Streaming transformation implemented
✅ Comprehensive unit tests pass
✅ Integration tests pass
✅ Detailed logging for debugging
✅ Documentation complete

---

## Future Enhancements

After basic proxy implementation is complete, consider:

- **Provider Proxy Integration:** Connect transformations to actual provider proxy logic
- **Tool/Function Calling:** Add support for Claude's tool use format
- **Vision Support:** Handle image content blocks
- **Error Transformation:** Map Claude errors to OpenAI error format
- **Prompt Caching:** Leverage Claude's prompt caching features
