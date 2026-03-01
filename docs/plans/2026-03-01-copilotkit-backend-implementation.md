# CopilotKit Phase 2 — Backend Implementation Log

> **Date:** 2026-03-01
> **Status:** Backend complete, frontend pending
> **Build:** ✅ `go build ./...` passes
> **Tests:** ✅ All `go test ./internal/...` pass

## Completed Tasks

### 1. `bot-portal-eq4` — SSE Streaming Chat Proxy ✅

**File:** `internal/api/chat_stream.go` (59 lines, new)

Added `streamChatResponse()` method to `Router` that pipes an upstream SSE stream to the client:

- Uses `bufio.Scanner` with a 512KB buffer (handles large `tool_calls` chunks)
- Sets proper SSE headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`, `X-Accel-Buffering: no`
- Flushes after each blank line (SSE event delimiter) using `http.Flusher`
- Terminates cleanly on `data: [DONE]` signal
- Preserves the existing non-streaming path in `chat.go` for backward compatibility

**Integration in `chat.go`:** When `chatReq.Stream == true` and the upstream returns 200, the handler now delegates to `streamChatResponse()` instead of `io.ReadAll()`.

---

### 2. `bot-portal-psn` — Extend ChatRequest with Tools/Stream ✅

**File:** `internal/api/chat.go` (modified)

Extended the `ChatRequest` struct:

```go
type ChatRequest struct {
    Model      string          `json:"model"`
    Messages   []ChatMessage   `json:"messages"`
    Stream     bool            `json:"stream,omitempty"`
    Tools      json.RawMessage `json:"tools,omitempty"`
    ToolChoice json.RawMessage `json:"tool_choice,omitempty"`
}
```

Updated the proxy payload construction to conditionally forward `stream`, `tools`, and `tool_choice` fields when present. Changed the `Accept` header to `text/event-stream` when streaming.

---

### 3. `bot-portal-ucx` — Default Model Selection ✅

**Files modified:**

| File | Change |
|------|--------|
| `internal/models/models.go` | Added `IsDefault bool` field to `Model` struct |
| `internal/store/sqlite.go` | Added `is_default INTEGER DEFAULT 0` ALTER migration for models table |
| `internal/store/models.go` | Updated all CRUD queries to include `is_default`; added `GetDefault()`, `SetDefault()`, `scanModel()` |
| `internal/api/models.go` | Added `handleModelDefault()` handler (GET + PUT `/api/models/default`) |
| `internal/api/router.go` | Added route `mux.HandleFunc("/api/models/default", r.handleModelDefault)` |

**Fallback chain in `resolveModelForCopilot()`:**

1. Requested model ID → look up in DB
2. `COPILOT_DEFAULT_MODEL_ID` env var → look up in DB
3. `ModelStore.GetDefault()` → model with `is_default=1`, else first model by `created_at ASC`
4. Error: `"No models configured. Add a model in Settings → Models first"`

**`SetDefault()` uses a transaction** to atomically clear all defaults then set the new one.

---

### 4. `bot-portal-e8l` — Copilot System Prompt ✅

**File:** `internal/api/copilot_runtime.go` (part of the runtime endpoint)

Implemented `buildSystemPrompt()` and `injectSystemPrompt()`:

- Dynamically queries agent, model, and auth config counts
- Counts running agents separately
- Describes Bot Portal's capabilities and available actions
- Injected as first message only if no system message already exists
- Overridable via `COPILOT_SYSTEM_PROMPT` env var

---

### 5. `bot-portal-q2y` — `/api/copilot/chat/completions` Endpoint ✅

**File:** `internal/api/copilot_runtime.go` (313 lines, new)

Implements the CopilotKit self-hosted runtime protocol:

| Handler | Route | Method |
|---------|-------|--------|
| `handleCopilotChat` | `/api/copilot/chat/completions` | POST |
| `handleCopilotInfo` | `/api/copilot/info` | GET |

**`handleCopilotChat` flow:**

1. Parse `CopilotChatRequest` (superset of OpenAI format)
2. `resolveModelForCopilot()` — fallback chain (see task 3)
3. `resolveAuthForModel()` — finds auth config, gets token (reuses Copilot OAuth refresh logic)
4. `injectSystemPrompt()` — prepends system message if absent
5. Build upstream payload with `model`, `messages`, `stream: true`, `tools`, `tool_choice`
6. Proxy to provider with correct auth headers (via `provider.GetAuthHeader`)
7. On 401/403 for Copilot auth: refresh token and retry once
8. Stream response via `streamChatResponse()` (SSE)
9. On non-200: forward error body

**`handleCopilotInfo`** returns available models and supported features (`streaming`, `tools`, `function_calling`).

---

## Duplicate Tasks Closed

Tasks `bot-portal-ef8.1` through `bot-portal-ef8.4` were children of the epic (`bot-portal-ef8`) that duplicated the standalone P1 tasks. All four closed as duplicates.

## Remaining Open Tasks (Frontend)

| Task | Description | Status |
|------|-------------|--------|
| `bot-portal-3cq` | Create CopilotProvider component (`runtimeUrl="/api/copilot"`) | Open — stub file exists |
| `bot-portal-10s` | Expose app state via `useCopilotReadable` | Open — stub file exists |
| `bot-portal-spy` | Agent CRUD actions via `useCopilotAction` | Open — stub file exists |
| `bot-portal-lbi` | Model CRUD actions | Open — stub file exists |
| `bot-portal-x19` | Auth config actions | Open — stub file exists |
| `bot-portal-s9w` | Navigation actions | Open — stub file exists |
| `bot-portal-kt1` | Final assembly: wire in `App.tsx` | Open |
| `bot-portal-ef8` | Epic: Phase 2 self-hosted runtime | Open (parent) |

**Frontend dependencies installed:** `@copilotkit/react-core@1.52.1`, `@copilotkit/react-ui@1.52.1`, `@copilotkit/runtime@1.52.1`

**Frontend stub files created in `web/src/copilot/`:**
- `CopilotProvider.tsx`
- `useAppContext.ts`
- `useAgentActions.ts`
- `useModelActions.ts`
- `useAuthConfigActions.ts`
- `useNavigationActions.ts`

## New API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/copilot/chat/completions` | CopilotKit streaming chat (SSE) |
| GET | `/api/copilot/info` | Runtime info (models, features) |
| GET | `/api/models/default` | Get the default model |
| PUT | `/api/models/default` | Set a model as default (`{"modelId":"..."}`) |

## Database Changes

- Added `is_default INTEGER DEFAULT 0` column to `models` table (auto-migrated via `sqlite.go` ALTER TABLE)

## Files Changed Summary

| File | Lines | Action |
|------|-------|--------|
| `internal/api/chat_stream.go` | 59 | **New** — SSE streaming proxy |
| `internal/api/copilot_runtime.go` | 313 | **New** — CopilotKit runtime endpoint |
| `internal/api/chat.go` | ~20 | Modified — extended ChatRequest, streaming branch |
| `internal/api/router.go` | ~5 | Modified — added 3 routes |
| `internal/api/models.go` | ~40 | Modified — added handleModelDefault |
| `internal/models/models.go` | ~1 | Modified — added IsDefault field |
| `internal/store/models.go` | ~80 | Modified — is_default in queries, GetDefault, SetDefault |
| `internal/store/sqlite.go` | ~1 | Modified — is_default migration |
| `web/package.json` | ~3 | Modified — added @copilotkit deps |
| `docs/plans/2026-03-01-copilotkit-integration-phase2.md` | ~175 | Modified — added 4 prerequisite tasks |

