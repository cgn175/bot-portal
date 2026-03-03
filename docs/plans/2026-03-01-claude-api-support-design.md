# Claude API Support - Design Document

**Date:** 2026-03-01
**Status:** Approved
**Goal:** Add native Claude API support to Bot Portal with dual-format chat endpoint

---

## Overview

Add support for Anthropic's Claude API alongside existing OpenAI-compatible providers. The implementation includes:
1. **New Claude-specific endpoint** at `/api/claude` for direct Claude API compatibility
2. **Adaptive chat endpoint** that auto-detects model format (OpenAI vs Claude) and transforms requests accordingly
3. **Model detection** based on model name prefix (`claude-*`)

This enables Bot Portal to work with Claude Code CLI and other Claude-native clients while maintaining backward compatibility with existing OpenAI-format clients.

---

## Architecture

### Current State

```
Client → /api/chat/completions → Go Backend → Provider (OpenAI format)
```

### Proposed State

```
┌─────────────────────────────────────────────────────────┐
│  Clients                                                │
│  - Claude Code CLI                                      │
│  - OpenAI-format clients                                │
│  - CopilotKit (via sidecar)                             │
└───────────┬─────────────────────────────────────────────┘
            │
            ├─────── /api/claude ──────────┐
            │                              │
            └─── /api/chat/completions ────┤
                                           │
                                           ▼
                            ┌──────────────────────────────┐
                            │  Request Format Detector     │
                            │  - Check model name          │
                            │  - Detect request structure  │
                            └──────────┬───────────────────┘
                                       │
                    ┌──────────────────┴────────────────────┐
                    │                                       │
                    ▼                                       ▼
          ┌─────────────────────┐             ┌────────────────────────┐
          │  Claude Format      │             │  OpenAI Format         │
          │  - Transform if     │             │  - Pass through or     │
          │    needed           │             │    transform if needed │
          └──────────┬──────────┘             └──────────┬─────────────┘
                     │                                   │
                     └──────────┬────────────────────────┘
                                │
                                ▼
                    ┌───────────────────────┐
                    │  Model Store          │
                    │  - Get model by ID    │
                    │  - Get auth config    │
                    └───────────┬───────────┘
                                │
                                ▼
                    ┌───────────────────────┐
                    │  Provider Proxy       │
                    │  - Add auth headers   │
                    │  - Forward request    │
                    └───────────┬───────────┘
                                │
                                ▼
                    ┌───────────────────────┐
                    │  Upstream Provider    │
                    │  (OpenAI, Anthropic,  │
                    │   Kimi, etc.)         │
                    └───────────────────────┘
```

---

## Component Design

### 1. Claude-Specific Endpoint (`/api/claude`)

**Route:** `POST /api/claude`

**Purpose:** Direct Claude API compatibility for Claude Code CLI and other Anthropic API clients.

**Request Format:**
```json
{
  "model": "claude-opus-4-6",
  "max_tokens": 1024,
  "messages": [
    {"role": "user", "content": "Hello"}
  ],
  "system": "You are a helpful assistant",
  "stream": true,
  "temperature": 0.7
}
```

**Required Headers:**
- `x-api-key` (extracted from auth config, not from client)
- `anthropic-version: 2023-06-01` (added by backend)
- `Content-Type: application/json`

**Response Format (Non-Streaming):**
```json
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
```

**Response Format (Streaming):**
```
event: message_start
data: {"type":"message_start","message":{...}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: message_stop
data: {"type":"message_stop"}
```

**Implementation:**
```go
// internal/api/claude.go
func (r *Router) handleClaudeMessages(w http.ResponseWriter, req *http.Request) {
    // 1. Parse Claude request
    var claudeReq ClaudeRequest

    // 2. Get model from database (lookup by model name)
    model := r.modelStore.GetByID(claudeReq.Model)

    // 3. Get auth config for Anthropic provider
    authConfig := r.getAuthConfigForProvider("anthropic")

    // 4. Add required Claude headers
    headers := map[string]string{
        "x-api-key": authConfig.APIKey,
        "anthropic-version": "2023-06-01",
        "Content-Type": "application/json",
    }

    // 5. Proxy to provider (no transformation needed - already Claude format)
    r.proxyToProvider(w, req, model, authConfig, claudeReq, headers)
}
```

### 2. Adaptive Chat Endpoint (`/api/chat/completions`)

**Current Behavior:** OpenAI format only

**New Behavior:** Auto-detect format based on model name and transform if needed

**Model Detection Strategy:**
```go
func isClaudeModel(modelName string) bool {
    return strings.HasPrefix(strings.ToLower(modelName), "claude-")
}
```

**Transformation Logic:**

```go
func (r *Router) handleChatCompletions(w http.ResponseWriter, req *http.Request) {
    var chatReq ChatRequest
    json.NewDecoder(req.Body).Decode(&chatReq)

    // Get model details
    model, _ := r.modelStore.GetByID(chatReq.Model)

    if isClaudeModel(model.ModelName) {
        // Transform OpenAI → Claude
        claudeReq := transformToClaudeFormat(chatReq)
        r.proxyAsClaudeRequest(w, req, model, claudeReq)
    } else {
        // Keep existing OpenAI logic
        r.proxyAsOpenAIRequest(w, req, model, chatReq)
    }
}
```

### 3. Request Format Transformation

**OpenAI → Claude Transformation:**

```go
type TransformationLogic struct {
    // System message handling
    // OpenAI: messages array with role="system"
    // Claude: top-level "system" parameter

    // Message structure
    // OpenAI: {role, content}
    // Claude: {role, content} (same, but system extracted)
}

func transformToClaudeFormat(openAIReq ChatRequest) ClaudeRequest {
    claudeReq := ClaudeRequest{
        Model:      openAIReq.Model,
        MaxTokens:  openAIReq.MaxTokens,
        Messages:   []ClaudeMessage{},
        Stream:     openAIReq.Stream,
    }

    // Extract system message
    for _, msg := range openAIReq.Messages {
        if msg.Role == "system" {
            claudeReq.System = msg.Content
        } else {
            claudeReq.Messages = append(claudeReq.Messages, ClaudeMessage{
                Role:    msg.Role,
                Content: msg.Content,
            })
        }
    }

    return claudeReq
}
```

**Claude → OpenAI Transformation (for responses):**

```go
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

func extractTextContent(content []ContentBlock) string {
    for _, block := range content {
        if block.Type == "text" {
            return block.Text
        }
    }
    return ""
}

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

### 4. Streaming Response Transformation

**Claude SSE → OpenAI SSE:**

```go
func transformClaudeStreamToOpenAI(claudeStream io.Reader, w http.ResponseWriter) {
    scanner := bufio.NewScanner(claudeStream)

    for scanner.Scan() {
        line := scanner.Text()

        if strings.HasPrefix(line, "event:") {
            eventType := strings.TrimPrefix(line, "event: ")
            continue
        }

        if strings.HasPrefix(line, "data:") {
            data := strings.TrimPrefix(line, "data: ")

            var claudeEvent ClaudeStreamEvent
            json.Unmarshal([]byte(data), &claudeEvent)

            if claudeEvent.Type == "content_block_delta" {
                // Transform to OpenAI format
                openAIChunk := OpenAIStreamChunk{
                    ID:      claudeEvent.ID,
                    Object:  "chat.completion.chunk",
                    Created: time.Now().Unix(),
                    Model:   "claude-opus-4-6",
                    Choices: []StreamChoice{
                        {
                            Index: 0,
                            Delta: Delta{
                                Content: claudeEvent.Delta.Text,
                            },
                        },
                    },
                }

                // Write as SSE
                fmt.Fprintf(w, "data: %s\n\n", toJSON(openAIChunk))
                w.(http.Flusher).Flush()
            }
        }
    }

    // Send [DONE]
    fmt.Fprintf(w, "data: [DONE]\n\n")
    w.(http.Flusher).Flush()
}
```

---

## Data Models

### Claude-Specific Types

```go
// internal/api/claude.go

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
    Metadata      *Metadata       `json:"metadata,omitempty"`
}

type ClaudeMessage struct {
    Role    string `json:"role"` // "user" or "assistant"
    Content string `json:"content"`
}

type ClaudeResponse struct {
    ID           string         `json:"id"`
    Type         string         `json:"type"` // "message"
    Role         string         `json:"role"` // "assistant"
    Content      []ContentBlock `json:"content"`
    Model        string         `json:"model"`
    StopReason   string         `json:"stop_reason"`
    StopSequence *string        `json:"stop_sequence"`
    Usage        Usage          `json:"usage"`
}

type ContentBlock struct {
    Type string `json:"type"` // "text"
    Text string `json:"text"`
}

type Usage struct {
    InputTokens  int `json:"input_tokens"`
    OutputTokens int `json:"output_tokens"`
}

type ClaudeStreamEvent struct {
    Type  string       `json:"type"` // "content_block_delta", "message_start", etc.
    Index int          `json:"index,omitempty"`
    Delta *ContentDelta `json:"delta,omitempty"`
}

type ContentDelta struct {
    Type string `json:"type"` // "text_delta"
    Text string `json:"text"`
}
```

### Extended Chat Types

```go
// internal/api/chat.go

// Extend existing ChatRequest to support optional fields
type ChatRequest struct {
    Model       string        `json:"model"`
    Messages    []ChatMessage `json:"messages"`
    MaxTokens   int           `json:"max_tokens,omitempty"`
    Stream      bool          `json:"stream,omitempty"`
    Temperature float64       `json:"temperature,omitempty"`
    Tools       []Tool        `json:"tools,omitempty"`
    ToolChoice  interface{}   `json:"tool_choice,omitempty"`
}
```

---

## Error Handling

### Claude-Specific Errors

**Anthropic Error Format:**
```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "messages.0.content: Input should be a valid string"
  }
}
```

**Transformation to OpenAI Format:**
```go
func transformClaudeError(claudeErr ClaudeError) OpenAIError {
    return OpenAIError{
        Error: ErrorDetail{
            Message: claudeErr.Error.Message,
            Type:    mapErrorType(claudeErr.Error.Type),
            Code:    mapErrorCode(claudeErr.Error.Type),
        },
    }
}

func mapErrorType(claudeType string) string {
    switch claudeType {
    case "invalid_request_error":
        return "invalid_request_error"
    case "authentication_error":
        return "invalid_api_key"
    case "rate_limit_error":
        return "rate_limit_exceeded"
    default:
        return "api_error"
    }
}
```

### Logging

**Log Request Details:**
```go
log.Printf("[Claude API] Request: model=%s, messages=%d, stream=%v",
    req.Model, len(req.Messages), req.Stream)
```

**Log Transformation Events:**
```go
log.Printf("[Format Transform] OpenAI → Claude: extracted system=%v, messages=%d",
    len(claudeReq.System) > 0, len(claudeReq.Messages))
```

**Log Provider Responses:**
```go
log.Printf("[Claude API] Response: id=%s, stop_reason=%s, tokens=%d",
    resp.ID, resp.StopReason, resp.Usage.OutputTokens)
```

---

## Testing Strategy

### Unit Tests

**File:** `internal/api/claude_test.go`

```go
func TestTransformOpenAIToClaudeFormat(t *testing.T) {
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
            },
            expected: ClaudeRequest{
                Model:  "claude-opus-4-6",
                System: "You are helpful",
                Messages: []ClaudeMessage{
                    {Role: "user", Content: "Hello"},
                },
            },
        },
    }
    // ...
}

func TestTransformClaudeResponseToOpenAI(t *testing.T) {
    // Test response transformation
}

func TestIsClaudeModel(t *testing.T) {
    tests := []struct {
        modelName string
        expected  bool
    }{
        {"claude-opus-4-6", true},
        {"claude-sonnet-4", true},
        {"gpt-4", false},
        {"gpt-4o", false},
    }
    // ...
}
```

### Integration Tests

**File:** `test/claude_integration_test.go`

```go
func TestClaudeEndpoint(t *testing.T) {
    // 1. Create Claude model in database
    // 2. Create Anthropic auth config
    // 3. Send request to /api/claude
    // 4. Verify response format
}

func TestAdaptiveChatWithClaudeModel(t *testing.T) {
    // 1. Create Claude model
    // 2. Send OpenAI-format request to /api/chat/completions
    // 3. Verify transformation happened
    // 4. Verify response is OpenAI format
}

func TestClaudeStreaming(t *testing.T) {
    // 1. Send streaming request
    // 2. Verify SSE events
    // 3. Verify message assembly
}
```

### Manual Testing with Claude Code CLI

**Setup:**
```bash
# 1. Add Anthropic auth config to Bot Portal
curl -X POST http://localhost:8080/api/auth-configs \
  -H "Content-Type: application/json" \
  -d '{
    "id": "anthropic-main",
    "name": "Anthropic API Key",
    "provider": "anthropic",
    "auth_type": "bearer_token",
    "credentials": {"api_key": "sk-ant-..."}
  }'

# 2. Models should auto-discover Claude models
# 3. Configure Claude Code CLI to use Bot Portal
export ANTHROPIC_BASE_URL=http://localhost:8080/api/claude
export ANTHROPIC_API_KEY=dummy  # Not used, but CLI requires it

# 4. Test with Claude Code
claude-code "List files in current directory"
```

**Verification:**
- Bot Portal logs should show Claude API requests
- Requests should include detailed logging
- Responses should be in Claude format
- Streaming should work correctly

---

## Implementation Plan Summary

### Phase 1: Core Claude Support
1. Add Claude data types (`ClaudeRequest`, `ClaudeResponse`)
2. Implement `/api/claude` endpoint
3. Add model detection (`isClaudeModel()`)
4. Test with direct Claude API calls

### Phase 2: Adaptive Chat
5. Add format transformation functions
6. Modify `/api/chat/completions` to detect and transform
7. Test OpenAI clients still work
8. Test Claude models via OpenAI-format requests

### Phase 3: Streaming
9. Implement Claude SSE → OpenAI SSE transformation
10. Test streaming with both formats

### Phase 4: Testing & Polish
11. Add comprehensive unit tests
12. Add integration tests
13. Test with Claude Code CLI
14. Add detailed logging
15. Update documentation

---

## Files to Create/Modify

### New Files
- `internal/api/claude.go` - Claude endpoint and transformation logic
- `internal/api/claude_test.go` - Unit tests
- `test/claude_integration_test.go` - Integration tests

### Modified Files
- `internal/api/router.go` - Add `/api/claude` route
- `internal/api/chat.go` - Add adaptive format detection
- `internal/provider/registry.go` - Ensure Anthropic provider configured
- `CLAUDE.md` - Document Claude API support

---

## Success Criteria

✅ `/api/claude` endpoint accepts Claude-format requests
✅ `/api/chat/completions` auto-detects Claude models and transforms
✅ Claude Code CLI works with Bot Portal as backend
✅ OpenAI-format clients still work (backward compatibility)
✅ Streaming works for both formats
✅ All tests pass
✅ Detailed logging for debugging

---

## Future Enhancements

- **Tool/Function Calling:** Add support for Claude's tool use format
- **Vision Support:** Handle image content blocks
- **Message Batches:** Support Claude's batch API
- **Prompt Caching:** Leverage Claude's prompt caching
- **Extended Context:** Support Claude's 200K token context
