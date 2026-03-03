# Consolidate Chat Completion Endpoints Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Merge `handleCopilotKitChat` and `handleChatCompletions` into a single unified `handleChatCompletions` endpoint that serves both the CopilotKit sidecar and the direct chat UI.

**Architecture:** Extract shared proxy logic into a reusable `proxyChatCompletion` method. `handleChatCompletions` becomes the single entry point, supporting both streaming and non-streaming modes, tools passthrough, optional system prompt injection, and model fallback resolution. The CopilotKit sidecar's `baseURL` is updated from `/api/copilotkit` to `/api` so it hits `/api/chat/completions`. The `/api/copilotkit/chat/completions` route is kept as an alias for backward compatibility.

**Tech Stack:** Go (net/http), Node.js sidecar (AI SDK `@ai-sdk/openai`)

---

### Task 1: Unify `ChatRequest` to support all fields

**Files:**
- Modify: `internal/api/chat.go:17-25`

**Step 1: Add tools fields to ChatRequest**

Add `Tools` and `ToolChoice` fields to `ChatRequest` so it can carry CopilotKit tool payloads. Remove `CopilotKitChatRequest` since `ChatRequest` now covers both use cases.

```go
type ChatRequest struct {
	Model       string          `json:"model,omitempty"`
	Messages    []ChatMessage   `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Tools       json.RawMessage `json:"tools,omitempty"`
	ToolChoice  json.RawMessage `json:"tool_choice,omitempty"`
}
```

Note: `Model` changes from required to `omitempty` to support CopilotKit requests that may omit it.

**Step 2: Run build to verify**

Run: `go build ./...`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/api/chat.go
git commit -m "refactor: add tools fields to ChatRequest for unified endpoint"
```

---

### Task 2: Extract shared proxy logic into `proxyChatCompletion`

**Files:**
- Modify: `internal/api/chat.go`

**Step 1: Create `proxyChatCompletion` method**

Extract the common proxy logic from both handlers into a single method. This method handles:
- Auth resolution (reuses existing `resolveAuthForModel`)
- Payload building (includes tools if present)
- Upstream proxy request
- Copilot 401/403 token refresh retry
- Streaming vs non-streaming response handling

```go
type chatProxyOptions struct {
	model          *models.Model
	messages       []ChatMessage
	stream         bool
	tools          json.RawMessage
	toolChoice     json.RawMessage
	maxTokens      int
	temperature    *float64
}

func (r *Router) proxyChatCompletion(w http.ResponseWriter, opts chatProxyOptions) {
	token, baseURL, copilotAuth, err := r.resolveAuthForModel(opts.model)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	payload := map[string]interface{}{
		"model":    opts.model.ModelIdentifier,
		"messages": opts.messages,
		"stream":   opts.stream,
	}
	if len(opts.tools) > 0 {
		payload["tools"] = json.RawMessage(opts.tools)
	}
	if len(opts.toolChoice) > 0 {
		payload["tool_choice"] = json.RawMessage(opts.toolChoice)
	}
	if opts.maxTokens > 0 {
		payload["max_tokens"] = opts.maxTokens
	}
	if opts.temperature != nil {
		payload["temperature"] = *opts.temperature
	}

	// ... marshal, proxy, retry, stream/respond (move existing code here)
}
```

**Step 2: Rewrite `handleChatCompletions` to use `proxyChatCompletion`**

```go
func (r *Router) handleChatCompletions(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var chatReq ChatRequest
	if err := json.NewDecoder(req.Body).Decode(&chatReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Resolve model: direct lookup first, then fallback chain
	model, err := r.resolveModel(chatReq.Model)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Claude model handling (existing logic)
	if isClaudeModel(model.ModelIdentifier) {
		// ... existing Claude transform (keep as-is)
		return
	}

	// Inject system prompt if X-Inject-System-Prompt header is set
	messages := chatReq.Messages
	if req.Header.Get("X-Inject-System-Prompt") == "true" {
		messages = r.injectSystemPrompt(messages)
	}

	r.proxyChatCompletion(w, chatProxyOptions{
		model:       model,
		messages:    messages,
		stream:      chatReq.Stream,
		tools:       chatReq.Tools,
		toolChoice:  chatReq.ToolChoice,
		maxTokens:   chatReq.MaxTokens,
		temperature: chatReq.Temperature,
	})
}
```

**Step 3: Run build and tests**

Run: `go build ./... && go test ./internal/api/...`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/api/chat.go
git commit -m "refactor: extract proxyChatCompletion for unified chat proxy"
```

---

### Task 3: Merge model resolution logic

**Files:**
- Modify: `internal/api/copilotkit_runtime.go:179-227`
- Modify: `internal/api/chat.go`

**Step 1: Rename `resolveModelForCopilot` to `resolveModel` and make it the single resolver**

The current `handleChatCompletions` does a simple `GetByID` lookup. The current `resolveModelForCopilot` has a fallback chain. Merge them:
- If model ID is provided and found → use it
- If model ID is provided but not found → fall through to defaults
- Fallback chain: CopilotKit settings → env var → `is_default` flag → first model

This works for both callers because direct chat always provides a model ID (so it hits the first path), and CopilotKit may omit it (so it falls through).

```go
// resolveModel resolves the model to use for a chat request.
// If modelID is provided and found, returns it directly.
// Otherwise falls through: CopilotKit settings → env → is_default → first model.
func (r *Router) resolveModel(modelID string) (*models.Model, error) {
	// ... same logic as current resolveModelForCopilot
}
```

**Step 2: Update `handleChatCompletions` to call `resolveModel`**

Already done in Task 2 step 2.

**Step 3: Run build and tests**

Run: `go build ./... && go test ./internal/api/...`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/api/chat.go internal/api/copilotkit_runtime.go
git commit -m "refactor: unify model resolution into single resolveModel method"
```

---

### Task 4: Update sidecar to point at `/api/chat/completions`

**Files:**
- Modify: `sidecar/src/index.ts:28-29` (change `baseURL`)
- Modify: `sidecar/src/backend-adapter.ts:32` (change URL)

**Step 1: Update AI SDK `baseURL` in `index.ts`**

Change from `/api/copilotkit` to `/api`:

```typescript
const backendOpenAI = createOpenAI({
  baseURL: `${config.backendUrl}/api`,
  apiKey: 'backend-managed',
});
```

This makes the AI SDK call `${backendUrl}/api/chat/completions` (standard OpenAI path).

**Step 2: Update `BackendChatAdapter` URL in `backend-adapter.ts`**

```typescript
const url = `${this.backendUrl}/api/chat/completions`;
```

**Step 3: Add system prompt header to sidecar requests**

The sidecar should send `X-Inject-System-Prompt: true` so the backend injects the Bot Portal system prompt for CopilotKit requests. Update `backend-adapter.ts`:

```typescript
headers: {
  'Content-Type': 'application/json',
  'X-Inject-System-Prompt': 'true',
},
```

**Step 4: Verify sidecar compiles**

Run: `cd sidecar && npx tsc --noEmit`
Expected: PASS

**Step 5: Commit**

```bash
git add sidecar/src/index.ts sidecar/src/backend-adapter.ts
git commit -m "refactor: point sidecar at unified /api/chat/completions endpoint"
```

---

### Task 5: Route `/api/copilotkit/chat/completions` as alias

**Files:**
- Modify: `internal/api/router.go:118`

**Step 1: Point the copilotkit route at the unified handler**

```go
// CopilotKit endpoints — chat uses the unified handler
mux.HandleFunc("/api/copilotkit/chat/completions", r.handleChatCompletions)
```

This ensures any existing clients hitting the old URL still work.

**Step 2: Commit**

```bash
git add internal/api/router.go
git commit -m "refactor: alias copilotkit chat route to unified handler"
```

---

### Task 6: Delete dead code

**Files:**
- Modify: `internal/api/copilotkit_runtime.go` — remove `handleCopilotKitChat`, `CopilotKitChatRequest`
- Modify: `internal/api/chat.go` — remove the inline auth resolution code (now in `proxyChatCompletion`)

**Step 1: Remove `CopilotKitChatRequest` struct and `handleCopilotKitChat` method**

Delete lines 17-145 from `copilotkit_runtime.go`. Keep `handleCopilotKitInfo`, `resolveModel` (renamed), `resolveAuthForModel`, `injectSystemPrompt`, and `buildSystemPrompt`.

**Step 2: Run build and tests**

Run: `go build ./... && go test ./internal/api/...`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/api/copilotkit_runtime.go internal/api/chat.go
git commit -m "refactor: remove duplicate CopilotKit chat handler and request type"
```

---

### Task 7: Update tests

**Files:**
- Modify: `internal/api/chat_test.go`

**Step 1: Add test for streaming mode**

Test that `handleChatCompletions` with `"stream": true` returns SSE format.

**Step 2: Add test for tools passthrough**

Test that `tools` and `tool_choice` fields are forwarded to the upstream payload.

**Step 3: Add test for system prompt injection**

Test that `X-Inject-System-Prompt: true` header causes a system message to be prepended.

**Step 4: Add test for model fallback resolution**

Test that when model ID is empty, the fallback chain resolves correctly.

**Step 5: Run all tests**

Run: `go test ./internal/api/... -v`
Expected: ALL PASS

**Step 6: Commit**

```bash
git add internal/api/chat_test.go
git commit -m "test: add tests for unified chat completions endpoint"
```

---

## Summary of changes

| Before | After |
|--------|-------|
| `handleChatCompletions` — non-streaming, no tools, direct model lookup | `handleChatCompletions` — streaming + non-streaming, tools, fallback model resolution |
| `handleCopilotKitChat` — streaming, tools, fallback chain, system prompt | Deleted (merged into above) |
| `CopilotKitChatRequest` — separate struct | Deleted (unified into `ChatRequest`) |
| `resolveModelForCopilot` | Renamed to `resolveModel`, used by both paths |
| Inline auth resolution in `handleChatCompletions` | Reuses `resolveAuthForModel` from copilotkit_runtime.go |
| Sidecar hits `/api/copilotkit/chat/completions` | Sidecar hits `/api/chat/completions` |
| System prompt always injected for copilotkit | System prompt injected when `X-Inject-System-Prompt: true` header present |
