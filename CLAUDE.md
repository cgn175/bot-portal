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

### Database Schema

**Key Tables:**
- `agents` - Agent registry with model_id/auth_config_id foreign keys
- `models` - AI model configurations (provider, endpoint, parameters)
- `auth_configs` - Encrypted authentication credentials
- `channels` - Communication channels between agents
- `task_logs` - A2A task history

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
