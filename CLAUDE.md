# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Test Commands

```bash
# Build the application
make build

# Run the application (builds first)
make run

# Run all tests
make test

# Run tests for a specific package
go test -v ./internal/store/...
go test -v ./internal/api/...
go test -v ./internal/crypto/...

# Run a specific test
go test -v ./internal/store/ -run TestModelStore_CreateAndGet

# Run database migrations only
go run cmd/portal/main.go --migrate-only

# Lint code (requires golangci-lint)
make lint

# Format code
make fmt

# Clean build artifacts and database
make clean

# Docker compose operations
make docker-run    # Start with docker-compose
make docker-down   # Stop docker-compose
```

## Architecture Overview

Bot Portal is a Go application with a React frontend that manages AI agents as Docker containers and routes messages between them using the Google A2A protocol.

### Layer Structure

```
cmd/portal/main.go
    → api.Router (HTTP handlers)
        → store.*Store (Database layer - SQLite)
        → docker.Manager (Docker API integration)
        → a2a.Router (A2A protocol message routing)
```

### Key Components

**Store Layer** (`internal/store/`)
- Uses SQLite with `modernc.org/sqlite` (pure Go driver)
- Each entity has its own store: `AgentStore`, `ModelStore`, `AuthConfigStore`, etc.
- Migrations run automatically on startup via `store.RunMigrations()`
- Foreign key relationships: agents reference models and auth_configs

**API Layer** (`internal/api/`)
- Standard library `net/http` with `http.NewServeMux()`
- Two API surfaces:
  - A2A Protocol endpoints (`/.well-known/agent.json`, `/tasks/*`) - for agent-to-agent communication
  - REST endpoints (`/api/*`) - for management UI
- Bearer token authentication for A2A endpoints via `requireBearerToken()` middleware

**Docker Manager** (`internal/docker/`)
- Integrates with Docker Engine API via `github.com/docker/docker`
- Creates/starts/stops agent containers
- Injects configuration as environment variables (MODEL_*, AUTH_*)
- Uses Docker context detection for macOS Docker Desktop

**A2A Router** (`internal/a2a/`)
- Implements Google A2A protocol
- Routes tasks between agents via channels (`agent1::agent2` or `general`)
- Stores task logs in database

**Crypto** (`internal/crypto/`)
- AES-256-GCM encryption for auth credentials
- Requires `ENCRYPTION_KEY` environment variable (32 bytes)
- Falls back to hardcoded default key (security issue - see code review)

**Provider Registry** (`internal/provider/`)
- Centralized provider definitions with metadata (base URL, headers, auth type)
- 25+ built-in providers including OpenAI, Anthropic, Kimi, DeepSeek, GLM, MiniMax, Qwen, etc.
- Provider-specific headers automatically applied (e.g., `User-Agent: KimiCLI/0.77` for Kimi)
- Regional variants supported (e.g., `kimi`, `kimi-cn`, `glm`, `glm-cn`)
- Custom provider support for any OpenAI-compatible endpoint

### Data Flow

1. **Agent Creation:**
   ```
   POST /api/agents → AgentStore.Create() → Database
   ```

2. **Agent Start with Model/Auth:**
   ```
   POST /api/agents/{id}?action=start
   → Router fetches Model + AuthConfig from stores
   → docker.Manager.CreateContainer() with env vars
   → Docker creates container with MODEL_*, AUTH_* env vars
   ```

3. **A2A Message Flow:**
   ```
   Agent → POST /tasks (with bearer token)
   → a2a.Router validates and routes
   → Forwards to recipient agent's /tasks endpoint
   → Logs to task_logs table
   ```

4. **Model Discovery Flow:**
   ```
   POST /api/auth-configs (with provider ID)
   → Creates auth config
   → Async model discovery via provider's /models endpoint
   → Provider-specific headers applied automatically
   → Saves discovered models to database
   ```

5. **Chat Completions Flow:**
   ```
   POST /api/chat/completions (with model ID)
   → Looks up model and associated auth config
   → Applies provider-specific headers
   → Proxies request to provider endpoint
   → Returns response

### Database Schema

**Key Tables:**
- `agents` - Agent registry with model_id/auth_config_id foreign keys
- `models` - AI model configurations (provider, endpoint, parameters)
- `auth_configs` - Encrypted authentication credentials
- `channels` - Communication channels between agents
- `task_logs` - A2A task history

**Cascading Behavior:**
- Deleting an `auth_config` automatically deletes all associated `models` (by provider)
- This prevents orphaned models when a provider configuration is removed

### Environment Variables

```bash
PORT=8080                    # Server port
DB_PATH=./bot-portal.db      # SQLite database path
DOCKER_NETWORK=bot-portal    # Docker network name
ENCRYPTION_KEY=...           # 32-byte AES key for credential encryption
```

### Frontend

React + TypeScript + Vite frontend in `web/` directory. Built files served from `web/dist/`.

```bash
cd web
npm install
npm run dev      # Development server
npm run build    # Production build
```

**Features:**
- **Agents**: Create, start, stop, restart Docker containers
- **Models**: View auto-discovered models from providers
- **Auth Configs**: Configure provider credentials (OAuth or API keys)
- **Test Chat**: Interactive chat interface to test models at `/test-chat`
- **Channels**: A2A communication channels between agents

## CopilotKit Architecture

The project uses a **Node.js sidecar** (`sidecar/`) to bridge the React frontend's CopilotKit integration to the Go backend.

### Why a sidecar?

CopilotKit expects a GraphQL endpoint with a specific schema. Rather than implement GraphQL from scratch in Go, we use a thin Node.js service with `@copilotkit/runtime` that:

1. Accepts GraphQL requests from React frontend
2. Transforms them to REST calls to Go backend's `/api/copilot/chat/completions`
3. Streams responses back as GraphQL subscriptions

### Running the sidecar

```bash
cd sidecar
npm install
npm run dev  # Port 3001
```

### Testing

```bash
./sidecar/test-sidecar.sh
```

Requires:
- Go backend running on port 8080
- Node.js sidecar running on port 3001

## Claude API Support

Bot Portal natively supports both OpenAI and Claude API formats.

**Endpoints:**
- `/api/claude` - Native Claude API format (for Claude Code CLI)
- `/api/chat/completions` - Adaptive format (auto-detects Claude models)

**Model Detection:** Any model name starting with `claude-` is automatically detected and transformed.

See `docs/claude-api.md` for full details.

## Testing Conventions

- Table-driven tests using Go's standard testing package
- In-memory SQLite (`:memory:`) for store tests
- `httptest` for API handler tests
- Test files named `*_test.go` alongside source files

## Common Tasks

**Add a new database table:**
1. Add CREATE TABLE to `internal/store/sqlite.go` migrations slice
2. Add struct to `internal/models/models.go`
3. Create store in `internal/store/{entity}.go`
4. Add tests in `internal/store/{entity}_test.go`

**Add a new API endpoint:**
1. Add handler method to `internal/api/router.go` or new file
2. Register route in `Router.Run()`
3. Add tests in `internal/api/{entity}_test.go`

**Run with local Docker:**
```bash
docker-compose up -d
```

The portal needs Docker socket access to manage agent containers.

## Supported AI Providers

The following providers are pre-configured with correct endpoints and headers:

### Popular Providers
| Provider | ID | Default URL | Notes |
|----------|-----|-------------|-------|
| OpenAI | `openai` | https://api.openai.com/v1 | Standard OpenAI API |
| Anthropic | `anthropic` | https://api.anthropic.com/v1 | Claude models |
| GitHub Copilot | `github_copilot` | https://api.githubcopilot.com | OAuth device flow |
| Google Gemini | `gemini` | https://generativelanguage.googleapis.com/v1beta | Google AI |

### Chinese Providers
| Provider | ID | Default URL | Special Headers |
|----------|-----|-------------|-----------------|
| Kimi (Moonshot) | `kimi` | https://api.moonshot.cn/v1 | User-Agent: KimiCLI/0.77 |
| Kimi Code | `kimi-code` | https://api.kimi.com/coding/v1 | User-Agent: KimiCLI/0.77 |
| DeepSeek | `deepseek` | https://api.deepseek.com | - |
| GLM (Zhipu) | `glm` | https://open.bigmodel.cn/api/paas/v4 | - |
| GLM China | `glm-cn` | https://open.bigmodel.cn/api/paas/v4 | China region |
| MiniMax | `minimax` | https://api.minimaxi.com/v1 | Global |
| MiniMax China | `minimax-cn` | https://api.minimaxi.cn/v1 | China region |
| Qwen (Dashscope) | `qwen` | https://dashscope.aliyuncs.com/compatible-mode/v1 | China |
| Qwen International | `qwen-intl` | https://dashscope-intl.aliyuncs.com/compatible-mode/v1 | International |
| Qwen Code | `qwen-code` | https://chat.qwen.ai/api | User-Agent: QwenCode/1.0 |
| Baidu Qianfan | `qianfan` | https://aip.baidubce.com | - |
| Z.AI | `zai` | https://api.z.ai/api/coding/paas/v4 | - |

### Open Source & Inference Providers
| Provider | ID | Default URL |
|----------|-----|-------------|
| Groq | `groq` | https://api.groq.com/openai/v1 |
| Together AI | `together` | https://api.together.xyz/v1 |
| Fireworks | `fireworks` | https://api.fireworks.ai/inference/v1 |
| Mistral | `mistral` | https://api.mistral.ai/v1 |
| Ollama | `ollama` | http://localhost:11434/v1 | Local |
| LM Studio | `lmstudio` | http://localhost:1234/v1 | Local |
| OpenRouter | `openrouter` | https://openrouter.ai/api/v1 |

### Enterprise Providers
| Provider | ID | Default URL |
|----------|-----|-------------|
| xAI (Grok) | `xai` | https://api.x.ai/v1 |
| Perplexity | `perplexity` | https://api.perplexity.ai |
| Cohere | `cohere` | https://api.cohere.com/compatibility/v1 |
| Cloudflare AI | `cloudflare` | https://gateway.ai.cloudflare.com/v1 |

### Custom Provider
For any OpenAI-compatible endpoint not listed above, use:
- **ID**: `custom`
- **URL**: Your custom endpoint URL

## Common Tasks

**Add a new provider:**
1. Add provider definition to `internal/provider/registry.go`
2. Include: ID, name, authType, defaultURL, apiKeyEnvVar, headers, aliases
3. Add to `Registry` map
4. Register any aliases in the `init()` function

Example:
```go
MyProvider = Provider{
    ID:           "myprovider",
    Name:         "My Provider",
    AuthType:     "bearer_token",
    AuthStyle:    AuthStyleBearer,
    DefaultURL:   "https://api.myprovider.com/v1",
    APIKeyEnvVar: "MYPROVIDER_API_KEY",
    Headers: map[string]string{
        "User-Agent": "BotPortal/1.0",
    },
    Description: "My AI provider",
    Aliases:     []string{"mp"},
}
```

**Add a new database table:**
1. Add CREATE TABLE to `internal/store/sqlite.go` migrations slice
2. Add struct to `internal/models/models.go`
3. Create store in `internal/store/{entity}.go`
4. Add tests in `internal/store/{entity}_test.go`

**Add a new API endpoint:**
1. Add handler method to `internal/api/router.go` or new file
2. Register route in `Router.Run()`
3. Add tests in `internal/api/{entity}_test.go`

**Run with local Docker:**
```bash
docker-compose up -d
```

The portal needs Docker socket access to manage agent containers.
