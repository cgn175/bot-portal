# Bot Portal MVP - Implementation Plan

## Overview
Build a web portal to manage multiple zeroclaw AI agents running as Docker containers. Portal serves as A2A message router with channels (agent1::agent2, general), message logging, and agent configuration.

**Tech Stack:**
- Backend: Go
- Frontend: React + TypeScript + Vite
- Database: SQLite
- Real-time: Server-Sent Events (SSE)
- Container Mgmt: Docker Engine API
- Protocol: Google A2A Standard

---

## Phase 1: Foundation

### Task 1.1: Project Scaffolding
- Initialize Go module with `go mod init`
- Create directory structure per ADR-001:
  - `cmd/portal/` - Entry point
  - `internal/api/` - HTTP handlers
  - `internal/a2a/` - A2A protocol
  - `internal/docker/` - Container management
  - `internal/store/` - SQLite storage
  - `internal/models/` - Domain models
  - `web/` - React frontend
  - `migrations/` - SQL migrations
- Create Makefile with build/run/test targets
- Create `docker-compose.yml` for dev environment

### Task 1.2: A2A Protocol Types
- Define Go types matching Google A2A standard:
  - `AgentCard` - Discovery document
  - `Task` - Unit of work with status, messages, artifacts
  - `TaskMessage` - Role, content, timestamp
  - `TaskUpdate` - SSE event
  - `Artifact` - File/image/data
  - `CreateTaskRequest/Response`
  - `A2APeer` and `A2AConfig` for peer management

---

## Phase 2: Data Layer

### Task 2.1: Data Models & SQLite Storage
- Define domain models:
  - `Agent` - ID, Name, Description, Image, Status, ContainerID, Endpoint, etc.
  - `Channel` - ID, Members, CreatedAt
  - `TaskLog` - Full task lifecycle with channel metadata
- Create SQLite migrations:
  - `001_create_agents.sql`
  - `001_create_channels.sql`
  - `001_create_task_logs.sql`
- Implement CRUD operations:
  - Agent queries (create, read, update, delete)
  - Channel queries
  - Message log queries

---

## Phase 3: Core Services

### Task 3.1: Docker Container Lifecycle Manager
- Implement Docker Engine API integration using Go SDK
- Support operations:
  - Create containers from agent images
  - Start/stop/restart/remove containers
  - Stream container logs
  - Monitor container health
- Configure environment variables:
  - `AGENT_ID` - Unique identifier
  - `PORTAL_URL` - For agent registration
  - `PORTAL_BEARER_TOKEN` - Auth token

### Task 3.2: A2A Message Router
**Server Side (receive tasks):**
- `POST /tasks` - Create task
- `GET /tasks/{id}` - Get task status
- `GET /tasks/{id}/stream` - SSE task updates
- `POST /tasks/{id}/cancel` - Cancel task

**Client Side (forward tasks):**
- `POST /tasks` - Forward to target agents
- `GET /tasks/{id}/stream` - Subscribe to agent updates

**Discovery:**
- Serve `GET /.well-known/agent.json` - Portal's AgentCard

**Routing:**
- `agent1::agent2` - Direct channel
- `general` - Broadcast to all agents
- Log all Tasks and TaskMessages

---

## Phase 4: REST API

### Task 4.1: Agent Management Endpoints
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/agents` | GET | List all agents |
| `/api/agents` | POST | Register new agent |
| `/api/agents/:id` | GET | Get agent details |
| `/api/agents/:id` | PUT | Update agent config |
| `/api/agents/:id` | DELETE | Remove agent |
| `/api/agents/:id/start` | POST | Start agent |
| `/api/agents/:id/stop` | POST | Stop agent |
| `/api/agents/:id/restart` | POST | Restart agent |
| `/api/agents/:id/logs` | GET (SSE) | Stream logs |

### Task 4.2: Channel & Message Endpoints
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/channels` | GET | List channels |
| `/api/channels/:id/messages` | GET | Message history |
| `/api/messages/stream` | GET (SSE) | Real-time stream |

### Task 4.3: Model & Auth Config Endpoints
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/models` | GET/POST | Manage centralized model configurations |
| `/api/models/:id` | GET/PUT/DELETE | Model specific operations |
| `/api/auth-configs` | GET/POST | Manage encrypted auth credentials |
| `/api/auth-configs/:id` | GET/PUT/DELETE | Auth config specific operations |

---

## Phase 5: Frontend

### Task 5.1: React Setup
- Initialize React + TypeScript + Vite
- Set up project structure:
  - `components/` - Reusable UI components
  - `pages/` - Page components
  - `hooks/` - Custom hooks
  - `api/` - API client

### Task 5.2: Dashboard Page
- Agent overview with status indicators
- Quick actions (start/stop/restart)
- Channel list

### Task 5.3: Agent Detail Page
- Configuration editor
- Container logs viewer
- AgentCard viewer

### Task 5.4: Message Viewer
- Real-time A2A message log
- Filter by channel
- Message details view

---

## Phase 6: Testing

### Task 6.1: Integration Testing
- Create mock agent Docker image for testing
- Test scenarios:
  - Agent registration
  - A2A message routing (direct & broadcast)
  - Channel creation
  - Message logging

---

## Dependencies Graph

```
bot-portal-wzv (Epic)
├── bot-portal-wzv.1: Project Scaffolding
│   └── go.mod, directory structure, Makefile, docker-compose.yml
├── bot-portal-wzv.6: A2A Protocol Types
│   └── blocks: wzv.1
├── bot-portal-wzv.2: Data Models & SQLite
│   └── blocks: wzv.1
├── bot-portal-wzv.3: Docker Manager
│   └── blocks: wzv.1
├── bot-portal-wzv.5: A2A Router
│   └── blocks: wzv.6, wzv.2
├── bot-portal-wzv.4: REST API
│   └── blocks: wzv.2, wzv.3, wzv.5
├── bot-portal-wzv.7: React Frontend
│   └── blocks: wzv.4
└── bot-portal-wzv.8: Integration Testing
    └── blocks: wzv.4
```

---

## Execution Order

1. **Scaffolding** → Foundation ready
2. **A2A Types** → Protocol definitions (can parallel with Scaffolding)
3. **Data Layer** → Database ready
4. **Docker Manager** → Container control ready
5. **A2A Router** → Core routing logic
6. **REST API** → HTTP layer
7. **Frontend** → User interface
8. **Testing** → Validation

---

## Notes

- All A2A messages must be logged and viewable
- Agent configuration (name, skills, env vars, A2A peers) managed through portal
- Portal acts as A2A registry - agents discover each other through it
- Bearer token auth for A2A communication
- SSE for real-time updates throughout the system

---

## Phase 7: Model & Auth Configs UI and Provider Integrations

### Task 7.1: Advanced Auth Providers (Backend)
- Add backend support for **GitHub Copilot Device Flow** (similar to zeroclaw's implementation):
  - `POST /api/auth/copilot/device-code` - Initiates flow, returns `device_code`, `user_code`, `verification_uri`.
  - `POST /api/auth/copilot/token` - Polls the token endpoint using the device code and saves the generated token as an encrypted Auth Config.
- Ensure `internal/docker/manager.go` injects the correct generic or specific provider environment variables (`API_KEY`, `ANTHROPIC_API_KEY`, `COPILOT_API_KEY`, etc.) based on the related Model Provider and Auth Config Type.

### Task 7.2: Models Management UI
- Create `/models` route in React app.
- **Model List**: Table showing `ID`, `Name`, `Provider`, `Identifier`.
- **Model Form**: Create/Edit form requiring:
  - `ID`: Unique identifier
  - `Name`: Human-readable name
  - `Provider`: Dropdown (OpenAI, Anthropic, Gemini, Copilot, Ollama, OpenRouter, etc.)
  - `Model Identifier`: E.g., `gpt-4o`, `claude-3-5-sonnet`
  - `Endpoint URL`: Optional override for compatible proxies
  - `Default Params`: JSON editor for `temperature`, `max_tokens`

### Task 7.3: Auth Configs Management UI
- Create `/auth-configs` route.
- **Auth Config List**: Table showing `ID`, `Name`, `Type`. Passwords/keys themselves remain masked/hidden.
- **Auth Config Form**: Create/Edit form requiring:
  - `ID`
  - `Name`
  - `Auth Type`: Dropdown (`Bearer Token`, `Basic Auth`, `GitHub Copilot OAuth`, etc.)
- **Dynamic Auth UIs**:
  - *Standard Providers (Bearer)*: Show an `API Key` password input, stored as `{"api_key": "..."}`.
  - *GitHub Copilot OAuth*: 
    - Show an "Authenticate with GitHub" button.
    - Modal opens with instructions, the `user_code`, and a link to `github.com/login/device`.
    - Modal polls backend until token is successfully retrieved and saved.
