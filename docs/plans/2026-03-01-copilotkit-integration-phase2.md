# CopilotKit Integration - Phase 2: Self-Hosted Runtime

> **Goal:** Integrate CopilotKit into the Bot Portal frontend with a **self-hosted runtime** that uses your existing models and auth configs from the database.

> **Tracking:** See Beads issues `bot-portal-iol` through `bot-portal-kt1`

## Why Self-Hosted Instead of Copilot Cloud?

| Aspect | Copilot Cloud (Phase 1) | Self-Hosted Runtime (Phase 2) |
|--------|------------------------|------------------------------|
| **Models** | Uses CopilotKit's LLM | Uses YOUR models from DB |
| **Auth** | Separate API key | Uses your existing auth configs |
| **Providers** | Limited to what they support | All 25+ providers you configured |
| **Cost** | External API calls | Your existing API keys |
| **Privacy** | Data goes to CopilotKit | Stays in your infrastructure |

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  React Frontend                                             │
│  ┌───────────────────────────────────────────────────────┐  │
│  │ <CopilotKit runtimeUrl="/api/copilot">                │  │
│  │   ┌──────────────┐  ┌──────────────────────────────┐  │  │
│  │   │ useCopilot*  │  │ CopilotPopup (chat UI)       │  │  │
│  │   │ hooks        │  │                              │  │  │
│  │   └──────────────┘  └──────────────────────────────┘  │  │
│  └─────────────────────┬─────────────────────────────────┘  │
│                        │                                     │
│  Existing API Client ──┼──► Go Backend                      │
└────────────────────────┼─────────────────────────────────────┘
                         │
                         ▼
              ┌─────────────────────┐
              │  /api/copilot/*     │
              │  CopilotRuntime     │
              │  (Go implementation)│
              └──────────┬──────────┘
                         │
         ┌───────────────┼───────────────┐
         ▼               ▼               ▼
   ┌──────────┐   ┌──────────┐   ┌──────────────┐
   │ ModelStore│   │AuthConfig│   │ Chat Handler │
   │ (SQLite) │   │  Store   │   │ (existing)   │
   └──────────┘   └──────────┘   └──────────────┘
```

## Key Differences from Phase 1 Plan

### Frontend Change

```diff
- <CopilotKit publicApiKey={VITE_COPILOT_CLOUD_API_KEY}>
+ <CopilotKit runtimeUrl="/api/copilot">
```

### Backend Addition

Implement CopilotRuntime protocol endpoints in Go:

```go
// internal/api/copilot_runtime.go
mux.HandleFunc("/api/copilot/chat/completions", r.handleCopilotChat)
```

## Beads Task Breakdown

| Issue | Task | Purpose |
|-------|------|---------|
| `bot-portal-iol` | **Research** | Can we implement CopilotRuntime natively in Go? |
| `bot-portal-ef8` | **Epic** | Phase 2: Self-Hosted Runtime |
| `bot-portal-4h5` | Install dependencies | `@copilotkit/react-core`, `@copilotkit/react-ui` |
| `bot-portal-3cq` | Create CopilotProvider | With `runtimeUrl` pointing to our backend |
| `bot-portal-eq4` | **SSE streaming chat proxy** | Add streaming support to chat proxy — prerequisite for `q2y` |
| `bot-portal-psn` | **Extend ChatRequest with tools/stream** | Forward `tools`, `tool_choice`, `stream` fields to upstream providers |
| `bot-portal-ucx` | **Default model selection** | Implement fallback chain: env var → flagged default → first model → error |
| `bot-portal-e8l` | **Copilot system prompt** | Design system prompt describing Bot Portal domain & available actions |
| `bot-portal-q2y` | Implement `/api/copilot/chat/completions` | Go backend endpoint (depends on `eq4`, `psn`, `ucx`, `e8l`) |
| `bot-portal-10s` | Expose app state | `useCopilotReadable` for agents, models, configs |
| `bot-portal-spy` | Agent actions | Create, delete, start, stop, restart, update |
| `bot-portal-lbi` | Model actions | Create, delete, sync |
| `bot-portal-x19` | Auth config actions | Create, delete |
| `bot-portal-s9w` | Navigation actions | Navigate between pages |
| `bot-portal-kt1` | Final assembly | Wire up in App.tsx |

## Implementation Order

```
Research (iol)
    │
    ▼
Install Deps (4h5) ──► CopilotProvider (3cq)
    │
    ▼
┌──────────────────────────────────────────────────┐
│  Backend Prerequisites (can be done in parallel) │
│                                                  │
│  SSE Streaming (eq4)                             │
│  Extend ChatRequest w/ tools (psn)               │
│  Default Model Selection (ucx)                   │
│  System Prompt Design (e8l)                      │
└──────────────────┬───────────────────────────────┘
                   │
                   ▼
         Runtime Endpoint (q2y)
                   │
                   ▼
    App Context (10s) ──► Actions (spy, lbi, x19, s9w)
                   │
                   ▼
           Final Assembly (kt1)
```

## CopilotRuntime Protocol

Based on CopilotKit's remote backend protocol, the runtime needs to handle:

### 1. Chat Completions (Streaming)

```
POST /api/copilot/chat/completions
Content-Type: application/json

{
  "model": "gpt-4o",  // Optional, use default if not provided
  "messages": [
    {"role": "system", "content": "You are a Bot Portal assistant..."},
    {"role": "user", "content": "List my agents"}
  ],
  "stream": true,
  "tools": [...]  // CopilotKit action definitions
}
```

Response: SSE stream (OpenAI-compatible format)

### 2. Runtime Info (Optional)

```
GET /api/copilot/info
```

Returns available models and features.

## Reusing Existing Infrastructure

The key insight: **CopilotKit's protocol is OpenAI-compatible**. We can reuse:

1. **`/api/chat/completions`** - Already implemented, proxies to providers
2. **ModelStore** - Already has model configs with base URLs
3. **AuthConfigStore** - Already has API keys
4. **Provider registry** - Already has 25+ providers with headers

### Implementation Strategy

```go
// internal/api/copilot_runtime.go
func (r *Router) handleCopilotChat(w http.ResponseWriter, req *http.Request) {
    // 1. Parse CopilotKit request
    var copilotReq CopilotChatRequest
    json.NewDecoder(req.Body).Decode(&copilotReq)

    // 2. Get default model from database (or use requested)
    model, authConfig := r.getDefaultModelForCopilot()

    // 3. Transform to standard chat completion request
    chatReq := ChatCompletionRequest{
        Model:    model.ModelName,
        Messages: copilotReq.Messages,
        Stream:   true,
    }

    // 4. Use existing chat completion handler logic
    r.proxyToProvider(w, req, model, authConfig, chatReq)
}
```

## New Prerequisite Tasks (Detail)

### `bot-portal-stm` — SSE Streaming Chat Proxy

**Problem:** The current `chat.go` reads the entire response body with `io.ReadAll()` and writes it back at once. CopilotKit **requires** SSE streaming (`text/event-stream`) for chat completions.

**Scope:**

1. Create `internal/api/chat_stream.go` with a new streaming proxy function
2. When `stream: true` is set in the request:
   - Add `"stream": true` to the upstream provider payload
   - Set response headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`
   - Pipe SSE chunks from the upstream provider back to the client using `http.Flusher`
   - Handle `data: [DONE]` termination signal
3. Preserve the existing non-streaming path for backward compatibility

**Key implementation detail:**

```go
// internal/api/chat_stream.go
func (r *Router) streamChatResponse(w http.ResponseWriter, upstreamResp *http.Response) error {
    flusher, ok := w.(http.Flusher)
    if !ok {
        return fmt.Errorf("streaming not supported")
    }

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.WriteHeader(http.StatusOK)

    scanner := bufio.NewScanner(upstreamResp.Body)
    for scanner.Scan() {
        line := scanner.Text()
        fmt.Fprintf(w, "%s\n", line)
        if line == "" {
            flusher.Flush()
        }
    }
    return scanner.Err()
}
```

**Files:**
- New: `internal/api/chat_stream.go`
- New: `internal/api/chat_stream_test.go`
- Modified: `internal/api/chat.go` (route streaming requests to new handler)

---

### `bot-portal-tfc` — Extend ChatRequest with Tools/Stream Fields

**Problem:** The current `ChatRequest` struct only has `Model` and `Messages`. CopilotKit sends `tools`, `tool_choice`, and `stream` in every request. These must be forwarded to the upstream LLM provider.

**Scope:**

1. Extend `ChatRequest` struct:
   ```go
   type ChatRequest struct {
       Model      string        `json:"model"`
       Messages   []ChatMessage `json:"messages"`
       Stream     bool          `json:"stream,omitempty"`
       Tools      json.RawMessage `json:"tools,omitempty"`
       ToolChoice json.RawMessage `json:"tool_choice,omitempty"`
   }
   ```
2. Update the proxy payload construction to include these fields when present
3. Handle `tool_calls` in streamed response chunks (CopilotKit uses these for actions)

**Files:**
- Modified: `internal/api/chat.go` (extend struct, forward fields)
- Modified: `internal/api/chat_stream.go` (handle tool_calls in stream chunks)

---

### `bot-portal-dms` — Default Model Selection Logic

**Problem:** `r.getDefaultModelForCopilot()` is referenced in the plan but has no defined behavior. Without a clear fallback chain, the endpoint will fail confusingly when no model is specified.

**Scope:**

1. Implement `getDefaultModelForCopilot()` with this fallback chain:
   - **Step 1:** Check `COPILOT_DEFAULT_MODEL_ID` env var → look up in ModelStore
   - **Step 2:** Query ModelStore for a model flagged as default (add `is_default` boolean column)
   - **Step 3:** Use the first model in the database (ordered by `created_at`)
   - **Step 4:** Return clear error: `"No models configured. Add a model in Settings → Models."`
2. Add `is_default` column to models table (migration)
3. Add "Set as Default" toggle in the Models UI

**Files:**
- New: `migrations/NNN_add_model_is_default.sql`
- Modified: `internal/models/models.go` (add `IsDefault` field)
- Modified: `internal/store/models.go` (add `GetDefault()`, update queries)
- Modified: `internal/api/copilot_runtime.go` (implement `getDefaultModelForCopilot()`)
- Modified: `web/src/pages/Models.tsx` (add default toggle)

---

### `bot-portal-spr` — Copilot System Prompt Design

**Problem:** CopilotKit sends the system prompt as part of messages, but the assistant needs domain context about Bot Portal to be useful. Without it, the LLM won't know what agents, models, or auth configs are.

**Scope:**

1. Create a system prompt template that describes:
   - What Bot Portal is (an AI agent management platform)
   - Available entities: Agents (docker/native, with status), Models (25+ providers), Auth Configs
   - Available actions the user can ask for (CRUD on agents, models, auth configs; navigation)
   - Current state summary (injected dynamically: agent count, model count, etc.)
2. Inject the system prompt as the first message if not already present
3. Make the prompt configurable via a `COPILOT_SYSTEM_PROMPT` env var override

**Implementation:**

```go
// internal/api/copilot_runtime.go
func (r *Router) buildSystemPrompt() string {
    agents, _ := r.agentStore.List()
    models, _ := r.modelStore.List()
    authConfigs, _ := r.authConfigStore.List()

    return fmt.Sprintf(`You are the Bot Portal assistant. Bot Portal manages AI agents, models, and auth configurations.

Current state:
- %d agents registered (%d running)
- %d models configured
- %d auth configurations

You can help users:
- List, create, start, stop, restart, and delete agents
- List, create, and delete models
- List, create, and delete auth configurations
- Navigate to different pages (Agents, Models, Auth, Chat, Messages)

When asked to perform an action, use the available tool functions.
Always confirm destructive actions (delete, stop) before executing.`,
        len(agents), countRunning(agents), len(models), len(authConfigs))
}
```

**Files:**
- Modified: `internal/api/copilot_runtime.go` (add `buildSystemPrompt()`, inject into messages)

---

## Environment Configuration

No API keys needed! The self-hosted runtime uses your existing database:

```bash
# No new env vars needed!
# It uses the same DB_PATH, and your existing models/auth_configs
```

Optional: Set default model for Copilot:

```bash
# .env (optional)
COPILOT_DEFAULT_MODEL_ID=my-default-model
```

## Files to Create/Modify

### New Files

```
web/src/copilot/
├── CopilotProvider.tsx      # runtimeUrl instead of publicApiKey
├── CopilotActions.tsx       # Renderless component
├── useAppContext.ts         # useCopilotReadable hooks
├── useAgentActions.ts       # Agent CRUD actions
├── useModelActions.ts       # Model CRUD actions
├── useAuthConfigActions.ts  # Auth config actions
└── useNavigationActions.ts  # Navigation actions

internal/api/
├── copilot_runtime.go       # Go implementation of CopilotRuntime
├── copilot_runtime_test.go  # Tests for runtime endpoint
├── chat_stream.go           # SSE streaming chat proxy
└── chat_stream_test.go      # Tests for streaming

migrations/
└── NNN_add_model_is_default.sql  # Add is_default column to models
```

### Modified Files

```
web/src/App.tsx              # Wrap with CopilotProvider
web/package.json             # Add @copilotkit/* dependencies
internal/api/router.go       # Add /api/copilot/* routes
internal/api/chat.go         # Extend ChatRequest, route streaming requests
internal/models/models.go    # Add IsDefault field to Model struct
internal/store/models.go     # Add GetDefault(), update queries for is_default
web/src/pages/Models.tsx     # Add "Set as Default" toggle
```

## Testing Plan

1. **Unit Tests**
   - `internal/api/chat_stream_test.go` — Test SSE streaming: mock upstream SSE response, verify chunks are flushed correctly, verify `data: [DONE]` terminates the stream
   - `internal/api/copilot_runtime_test.go` — Test request transformation, model selection fallback chain, system prompt injection, tool/tool_choice forwarding
   - `internal/store/models_test.go` — Test `GetDefault()` returns the flagged default, falls back to first model

2. **Automated Integration Test**
   - Send a CopilotKit-format POST to `/api/copilot/chat/completions` with `stream: true`, `tools: [...]`, and `messages: [...]`
   - Validate response is `text/event-stream` content type
   - Validate SSE chunks are valid OpenAI-compatible format
   - Validate tool_calls appear in response when tools are provided

3. **Manual Integration Test**: Start portal, open chat
   - Ask "What agents do I have?"
   - Ask to create a test agent
   - Verify it uses your configured model
   - Verify streaming responses render incrementally in CopilotKit UI

## Future Enhancements

1. **Multi-model Support** - Let Copilot use different models for different tasks
2. **Custom Tools** - Expose more complex actions as function calling
3. **Generative UI** - Render confirmation dialogs in chat
4. **Chat Persistence** - Save conversation history

---

## Comparison: Native Go vs Node.js Sidecar

| Approach | Complexity | Maintenance | Recommendation |
|----------|-----------|-------------|----------------|
| **Native Go** | Higher | Lower | **✓ Use this** - You're already using Go |
| **Node.js Sidecar** | Lower | Higher | Would need to maintain separate service |

The CopilotRuntime protocol is essentially a thin wrapper around OpenAI's chat completions. Implementing it natively in Go is straightforward and keeps your stack consistent.
