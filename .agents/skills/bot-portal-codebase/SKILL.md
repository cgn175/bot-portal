---
name: bot-portal-codebase
description: "Comprehensive codebase map for Bot Portal. Use at the START of any conversation about this codebase to avoid exploratory file reads. Provides exact file paths, function signatures, type definitions, API routes, and architecture for all Go backend, React frontend, and Node.js sidecar code."
---

# Bot Portal Codebase Map

**Module:** `github.com/zeroclaw/bot-portal` | **Go 1.25.4** | **SQLite (modernc.org/sqlite)** | **Docker SDK** | **React 18 + Vite + CopilotKit**

For detailed function signatures, database schema, and component reference, read `reference/detailed-map.md`.

## Quick Reference

| What | Where |
|------|-------|
| Entry point | `cmd/portal/main.go` → `main()` |
| HTTP router & all routes | `internal/api/router.go` → `Router.Run()` |
| Data models (structs) | `internal/models/models.go` |
| Database init + migrations | `internal/store/sqlite.go` → `NewSQLite()`, `RunMigrations()` |
| Docker container management | `internal/docker/manager.go` → `Manager` |
| A2A protocol types & router | `internal/a2a/types.go`, `internal/a2a/router.go` |
| AI provider registry (28) | `internal/provider/registry.go` |
| Encryption (AES-256-GCM) | `internal/crypto/encryption.go` |
| React frontend | `web/src/` (Vite + React Router + CopilotKit) |
| CopilotKit sidecar | `sidecar/src/` (Node.js + TypeScript) |
| Build & run | `Makefile` — `make build`, `make run`, `make test`, `make dev-all` |
| Docker Compose | `docker-compose.yml` — portal (8080), copilot-sidecar (3001) |
| Tests | `go test ./...` or `make test` |
| Database file | `db/portal.db` (SQLite WAL mode) |

## Architecture

```
React Frontend (port 5173) ──GraphQL+SSE──▶ Node.js Sidecar (port 3001) ──HTTP──▶ Go Backend (port 8080) ──▶ LLM Providers
                                                                                    │
                                                                              SQLite + Docker Engine
                                                                                    │
                                                                        Agent Containers (port 17000+)
```

## Environment Variables

| Var | Default | Purpose |
|-----|---------|---------|
| `DB_PATH` | `db/portal.db` | SQLite database file path |
| `PORT` | `8080` | HTTP server port |
| `ENCRYPTION_KEY` | *(required)* | 32-byte key for AES-256-GCM credential encryption |
| `ALLOWED_ORIGINS` | `localhost:3000,5173` | CORS allowed origins (comma-separated) |
| `PORTAL_INTERNAL_URL` | `http://host.docker.internal:8080` | Portal URL accessible from containers |
| `AGENT_CONFIG_DIR_HOST` | `${PWD}/agent-configs` | Host path for bind mounts |
| `SHARED_VOLUME_HOST_PATH` | *(empty)* | Shared workspace bind mount for agents |
| `COPILOT_SYSTEM_PROMPT` | *(auto-generated)* | Override CopilotKit system prompt |
| `REDACT_PATTERNS` | *(defaults)* | Comma-separated regex for log secret redaction |
| `MAX_LOG_STREAMS_PER_USER` | `5` | Max concurrent log streams per IP |
| `MAX_LOG_STREAMS_GLOBAL` | `20` | Max concurrent log streams globally |

## All HTTP Routes

| Method | Path | Handler Location |
|--------|------|-----------------|
| GET | `/.well-known/agent.json` | `api/router.go` → A2A agent card |
| POST | `/tasks` | `api/tasks.go` (bearer auth) |
| * | `/tasks/` | `api/tasks.go` (bearer auth) |
| * | `/a2a/relay/` | `api/tasks.go` (bearer auth) |
| GET/POST | `/api/agents` | `api/agents/handler.go` → list/create |
| * | `/api/agents/{id}` | `api/agents/handler.go` → get/update/delete + actions |
| * | `/api/agents/{id}?action=start\|stop\|restart\|recreate\|ping\|chat\|logs\|sync-peers` | `api/agents/lifecycle.go`, `peers.go` |
| GET | `/api/agents/{id}/identity-files` | `api/agents/identity.go` |
| GET/PUT | `/api/agents/{id}/identity-files/{filename}` | `api/agents/identity.go` |
| GET | `/api/agents/container-logs-stream?agentID=xxx` | `api/agents/logs.go` (SSE) |
| GET | `/api/agents-stream` | `api/agents/handler.go` (SSE, 3s poll) |
| GET/POST | `/api/channels` | `api/channels.go` |
| GET | `/api/channels/{id}` | `api/channels.go` |
| GET | `/api/messages/stream` | `api/router.go` (SSE) |
| GET | `/api/tasks/{id}/stream` | `api/router.go` (SSE for chat) |
| GET/POST/DELETE | `/api/models` | `api/models.go` |
| GET/PUT/DELETE | `/api/models/{id}` | `api/models.go` |
| GET/POST | `/api/auth-configs` | `api/auth_configs.go` |
| GET/PUT/DELETE | `/api/auth-configs/{id}` | `api/auth_configs.go` |
| POST | `/api/auth/copilot/device-code` | `api/copilot_auth.go` |
| POST | `/api/auth/copilot/token` | `api/copilot_auth.go` |
| GET | `/api/auth/copilot/models` | `api/copilot_auth.go` |
| GET | `/api/auth/discover-models?configId=xxx` | `api/auth_configs.go` |
| POST | `/api/chat/completions` | `api/chat.go` |
| POST | `/api/copilotkit/chat/completions` | `api/chat.go` (alias) |
| GET | `/api/copilotkit/info` | `api/copilotkit_runtime.go` |
| GET | `/api/providers` | `api/providers.go` |
| GET | `/api/providers/{id}` | `api/providers.go` |
| POST | `/api/claude` | `api/claude.go` (**501 stub**) |
| GET | `/health` | `api/router.go` (inline "OK") |

## Directory Structure

```
bot-portal/
├── cmd/portal/main.go              # Entry point
├── internal/
│   ├── api/                        # HTTP handlers
│   │   ├── router.go               # Main router, middleware, auth
│   │   ├── agents/                 # Agent sub-handlers
│   │   │   ├── handler.go          # Agent CRUD + dispatch
│   │   │   ├── lifecycle.go        # start/stop/restart/recreate/chat
│   │   │   ├── identity.go         # Identity file CRUD
│   │   │   ├── peers.go            # Peer sync + config regen
│   │   │   └── logs.go             # Container log SSE streaming
│   │   ├── auth_configs.go         # Auth config CRUD + model discovery
│   │   ├── chat.go                 # Chat completions proxy
│   │   ├── chat_stream.go          # SSE stream relay
│   │   ├── claude.go               # Claude API (501 stub)
│   │   ├── channels.go             # Channel CRUD
│   │   ├── copilot_auth.go         # GitHub Copilot OAuth device flow
│   │   ├── copilotkit_runtime.go   # CopilotKit info + model/auth resolution
│   │   ├── models.go               # Model CRUD
│   │   ├── providers.go            # Provider registry API
│   │   └── tasks.go                # A2A task handling + relay
│   ├── a2a/                        # A2A protocol
│   │   ├── types.go                # All A2A types + ChannelID()
│   │   └── router.go               # A2A message routing + SSE
│   ├── crypto/encryption.go        # AES-256-GCM encryption
│   ├── docker/
│   │   ├── manager.go              # Docker lifecycle + container creation
│   │   └── workspace_files.go      # Read/write files in containers
│   ├── models/models.go            # Shared data model structs
│   ├── provider/registry.go        # 28 AI provider definitions
│   └── store/                      # Database layer
│       ├── sqlite.go               # DB init + migrations
│       ├── agents.go               # AgentStore (has own Agent struct)
│       ├── auth_configs.go         # AuthConfigStore (auto-encrypt/decrypt)
│       ├── channels.go             # ChannelStore
│       ├── identity_files.go       # IdentityFileStore
│       ├── messages.go             # MessageStore (task_logs)
│       └── models.go               # ModelStore
├── web/src/                        # React frontend
│   ├── api/client.ts               # API client
│   ├── components/                 # Reusable components
│   ├── contexts/AgentContext.tsx    # Agent state context
│   ├── copilot/                    # CopilotKit hooks + actions
│   ├── hooks/useIdentityFiles.ts   # Identity file hook
│   ├── pages/                      # Route page components
│   ├── App.tsx                     # Layout + routes
│   └── main.tsx                    # React root
├── sidecar/src/                    # CopilotKit Node.js sidecar
│   ├── index.ts                    # Express server
│   ├── config.ts                   # Config
│   ├── backend-adapter.ts          # HTTP adapter to Go backend
│   └── runtime-adapter.ts          # CopilotRuntime config
├── test/                           # Integration tests + mock agent
├── scripts/                        # generate-key.sh, launch.sh
├── Makefile                        # build, run, test, dev-all, lint, fmt
└── docker-compose.yml              # portal + sidecar containers
```

## Key Patterns & Conventions

- **No web framework** — `net/http` stdlib (`http.NewServeMux`)
- **No ORM** — raw SQL with `database/sql`, parameterized queries
- **Store pattern** — `NewXxxStore(db) → *XxxStore` with CRUD methods
- **"Not found" = nil** — stores return `(nil, nil)` for missing records, NOT error
- **`store.ErrNotFound`** — sentinel error for update/delete when row doesn't exist
- **Agent types** — `"docker"` (managed containers) or `"native"` (external, always running)
- **Container naming** — `bot-portal-agent-{agentID}`
- **Listen port allocation** — `17000 + count(existing agents)`
- **Channel ID format** — Direct: `agent1::agent2` (sorted); Broadcast: `general`
- **Identity files** — Allowed: `IDENTITY.md`, `SOUL.md`, `AGENTS.md`, `USER.md`, `TOOLS.md`; max 16KB
- **Credential encryption** — auto encrypt/decrypt in AuthConfigStore, AES-256-GCM via `ENCRYPTION_KEY`
- **SSE streaming** — `text/event-stream` with `http.Flusher`, keepalive heartbeats
- **Background tasks** — tracked via `r.bgWg sync.WaitGroup` for graceful shutdown
- **CORS** — middleware reads `ALLOWED_ORIGINS`, defaults to localhost:3000/5173
- **Bearer auth** — constant-time token comparison, auto-sets `X-Agent-ID` from token
- **Model discovery** — auto-fetches `/models` endpoint on auth config create/update
- **Copilot token refresh** — auto-refreshes within 5min of expiry

## Development Commands

```bash
make build                       # Build Go binary to bin/bot-portal
make run                         # Build + run
make test                        # go test -v ./...
make dev-all                     # Start backend + sidecar + frontend
make lint                        # golangci-lint
make fmt                         # go fmt + go vet
make db-reset                    # Delete database file
cd web && npm run dev            # Frontend only (localhost:5173)
cd sidecar && npm run dev        # Sidecar only (localhost:3001)
docker-compose up -d             # Docker deployment
```
