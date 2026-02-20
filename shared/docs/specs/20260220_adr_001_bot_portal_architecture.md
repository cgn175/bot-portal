# ADR-001: Bot Portal Architecture

## Status
Proposed

## Context
We need a web-based portal to manage multiple zeroclaw AI agents. Each agent runs as a Docker container and communicates via zeroclaw's custom A2A protocol. The portal serves as:
1. A **messaging router/hub** for agent-to-agent communication
2. A **management UI** for agent lifecycle (start/stop/configure)
3. A **log viewer** for all inter-agent messages

### Key Requirements
- Agents are identified by unique IDs (peer IDs in zeroclaw config)
- Communication channels: `agent1-id::agent2-id` (direct) or `general` (broadcast)
- All A2A messages must be logged and viewable
- Agent configuration (name, skills, env vars, A2A peers) managed through the portal
- Portal acts as the A2A registry — agents discover each other through it

### ZeroClaw A2A Protocol (from `~/projects/zeroclaw`)
Zeroclaw implements the **Google A2A Protocol** standard:
- **Transport**: HTTP/HTTPS with SSE for streaming
- **Discovery**: `GET /.well-known/agent.json` — returns AgentCard (name, description, capabilities, skills, endpoints)
- **Create Task**: `POST /tasks` — creates a task with a `CreateTaskRequest` (message + optional metadata)
- **Get Task**: `GET /tasks/{id}` — returns task status and result
- **Stream Updates**: `GET /tasks/{id}/stream` — SSE stream of `TaskUpdate` events
- **Cancel Task**: `POST /tasks/{id}/cancel` — cancel a running task
- **Auth**: Bearer token per peer (static, configured in TOML)
- **Port**: 9000 (configurable via `listen_port`)
- **Config**: TOML at `~/.zeroclaw/config.toml` under `[channels_config.a2a]`

#### Key Protocol Types (from `src/channels/a2a/protocol.rs`)

**AgentCard** — Discovery document at `/.well-known/agent.json`:
```json
{
  "name": "Agent Name",
  "description": "What this agent does",
  "version": "0.1.0",
  "capabilities": { "streaming": true, "artifacts": true, "push_notifications": false },
  "authentication": { "schemes": ["bearer"] },
  "endpoints": { "tasks": "/tasks", "stream": "/tasks/{id}/stream" },
  "skills": [{ "id": "skill-id", "name": "Skill", "description": "..." }]
}
```

**Task** — Unit of work:
```json
{
  "id": "task-123",
  "status": "pending|running|completed|failed|cancelled",
  "created_at": "ISO8601",
  "updated_at": "ISO8601",
  "messages": [{ "role": "user|agent", "content": "...", "timestamp": "ISO8601" }],
  "artifacts": [{ "id": "art-1", "type": "file|image|data", "name": "...", "content": "..." }]
}
```

**TaskUpdate** — SSE event:
```json
{ "task_id": "task-123", "status": "running", "message": {...}, "artifact": {...} }
```

**CreateTaskRequest**:
```json
{ "message": { "role": "user", "content": "Do something" }, "metadata": {} }
```

#### Agent Config Structure (TOML)
```toml
[channels_config.a2a]
enabled = true
listen_port = 9000
discovery_mode = "static"
allowed_peer_ids = ["*"]

[channels_config.a2a.agent_card]
name = "My Agent"
description = "What this agent does"

[[channels_config.a2a.agent_card.skills]]
id = "code-review"
name = "Code Review"
description = "Review code for quality and security"

[[channels_config.a2a.peers]]
id = "agent-alpha"
endpoint = "https://192.168.1.100:9000"
bearer_token = "token-here"
enabled = true

[channels_config.a2a.rate_limit]
requests_per_minute = 60
burst_size = 10
```

## Decision

### Tech Stack

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| Backend API | **Go** | Team standard for microservices; excellent Docker SDK; strong concurrency for message routing |
| Frontend | **React + TypeScript + Vite** | Rich interactivity needed for real-time log viewer, agent management UI |
| Database | **SQLite** (MVP) → **PostgreSQL** (scale) | Simple start, no infra overhead; easy migration path |
| Real-time | **Server-Sent Events (SSE)** | A2A already uses SSE for streaming; consistent pattern; simpler than WebSocket for unidirectional updates |
| Container Mgmt | **Docker Engine API** (Go SDK) | Direct container lifecycle control |
| A2A Protocol | **Google A2A Standard** (HTTP + SSE) | Task-based model; AgentCard discovery; portal is both A2A peer and router |

### Architecture Overview

```
┌─────────────────────────────────────────────────┐
│                   Bot Portal                     │
│                                                  │
│  ┌──────────┐  ┌──────────┐  ┌───────────────┐  │
│  │ React UI │──│ REST API │──│ A2A Router    │  │
│  │ (Vite)   │  │ (Go)     │  │ (Go)          │  │
│  └──────────┘  └──────────┘  └───────────────┘  │
│                      │              │            │
│                ┌─────┴─────┐  ┌─────┴─────┐     │
│                │  SQLite   │  │  Docker   │     │
│                │  (store)  │  │  Engine   │     │
│                └───────────┘  └───────────┘     │
│                                     │            │
│              ┌──────────────────────┤            │
│              ▼          ▼           ▼            │
│         ┌────────┐ ┌────────┐ ┌────────┐        │
│         │Agent A │ │Agent B │ │Agent C │        │
│         │(Docker)│ │(Docker)│ │(Docker)│        │
│         └────────┘ └────────┘ └────────┘        │
└─────────────────────────────────────────────────┘
```

### Component Breakdown

#### 1. A2A Router (Core)
The portal acts as a **Google A2A peer and task/message router**:
- **Receives tasks**: Agents create tasks on portal via `POST /tasks` (portal is itself an A2A server)
- **Forwards tasks**: Portal creates tasks on target agents via `POST /tasks` on their endpoints
- **Streams updates**: Portal subscribes to `GET /tasks/{id}/stream` on agents for TaskUpdate SSE events
- **Serves own stream**: Portal exposes `GET /tasks/{id}/stream` for agents subscribing to task updates
- **Discovery**: Portal serves its own AgentCard at `GET /.well-known/agent.json`
- **Channel system**: Routes messages based on channel format:
  - `agentA-id::agentB-id` — direct channel, task routed to specific agent
  - `general` — broadcast task to all registered agents
- **Message logging**: Every Task, TaskMessage, and TaskUpdate passing through is persisted with channel metadata
- **Health monitoring**: Fetches each agent's `GET /.well-known/agent.json` to verify liveness

#### 2. Agent Registry & Discovery
- Portal maintains a registry of all agents and their AgentCards
- When an agent is registered, portal auto-generates bearer tokens and configures the agent's `config.toml` `[channels_config.a2a]` section (peers, agent_card, skills)
- Portal fetches each agent's AgentCard at `GET /.well-known/agent.json` to populate skills and capabilities
- Portal exposes `GET /api/agents` — returns all registered agents, their AgentCards, and status

#### 3. Agent Lifecycle Manager
- Uses Docker Engine API (Go SDK) to:
  - Create containers from agent images
  - Start/stop/restart containers
  - Stream container logs
  - Monitor container health
- Each agent container gets:
  - `PORTAL_URL` env var (to register back)
  - `AGENT_ID` env var (unique identifier)
  - Network connectivity to portal

#### 4. REST API (Portal Management)
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/agents` | GET | List all agents |
| `/api/agents` | POST | Register new agent |
| `/api/agents/:id` | GET | Get agent details + card |
| `/api/agents/:id` | PUT | Update agent config |
| `/api/agents/:id` | DELETE | Remove agent |
| `/api/agents/:id/start` | POST | Start agent container |
| `/api/agents/:id/stop` | POST | Stop agent container |
| `/api/agents/:id/restart` | POST | Restart agent container |
| `/api/agents/:id/logs` | GET (SSE) | Stream container logs |
| `/api/channels` | GET | List all channels |
| `/api/channels/:id/messages` | GET | Get channel message history |
| `/api/messages/stream` | GET (SSE) | Real-time message stream (portal UI) |
| `/.well-known/agent.json` | GET | Portal's own AgentCard (A2A discovery) |
| `/tasks` | POST | Create task (A2A standard — agents send tasks here) |
| `/tasks/:id` | GET | Get task status/result (A2A standard) |
| `/tasks/:id/stream` | GET (SSE) | Subscribe to task updates (A2A standard) |
| `/tasks/:id/cancel` | POST | Cancel a running task (A2A standard) |

#### 5. Frontend (React)
- **Dashboard**: Overview of all agents, their status (running/stopped/error)
- **Agent Detail**: Config editor, container logs, agent card viewer
- **Message Viewer**: Real-time log of all A2A messages, filterable by channel
- **Channel View**: View specific channel conversations

### Data Models

#### Agent
```go
type Agent struct {
    ID          string            `json:"id"`          // zeroclaw peer ID
    Name        string            `json:"name"`        // from AgentCard.name
    Description string            `json:"description"` // from AgentCard.description
    Image       string            `json:"image"`       // Docker image
    Status      string            `json:"status"`      // running, stopped, error
    ContainerID string            `json:"containerId"`
    Endpoint    string            `json:"endpoint"`    // A2A base URL (e.g. http://container:9000)
    ListenPort  int               `json:"listenPort"`  // A2A listen port (default 9000)
    BearerToken string            `json:"-"`           // never exposed in API responses
    AgentCard   *AgentCard        `json:"agentCard"`   // cached AgentCard from discovery
    Config      map[string]string `json:"config"`      // env vars / settings
    CreatedAt   time.Time         `json:"createdAt"`
    UpdatedAt   time.Time         `json:"updatedAt"`
}
```

#### AgentCard (mirrored from zeroclaw protocol.rs)
```go
type AgentCard struct {
    Name           string             `json:"name"`
    Description    string             `json:"description"`
    Version        string             `json:"version"`
    Capabilities   AgentCapabilities  `json:"capabilities"`
    Authentication AuthenticationInfo `json:"authentication"`
    Endpoints      AgentEndpoints     `json:"endpoints"`
    Skills         []Skill            `json:"skills"`
}
```

#### Channel
```go
type Channel struct {
    ID        string    `json:"id"`        // "agentA::agentB" or "general"
    Members   []string  `json:"members"`   // agent peer IDs
    CreatedAt time.Time `json:"createdAt"`
}
```

#### TaskLog (mirrors Google A2A Task lifecycle)
```go
type TaskLog struct {
    ID          string          `json:"id"`          // Task ID
    ChannelID   string          `json:"channelId"`   // derived: "sender::recipient" or "general"
    SenderID    string          `json:"senderId"`    // agent that created the task
    RecipientID string          `json:"recipientId"` // agent that received the task
    Status      string          `json:"status"`      // pending, running, completed, failed, cancelled
    Messages    json.RawMessage `json:"messages"`    // []TaskMessage JSON
    Artifacts   json.RawMessage `json:"artifacts"`   // []Artifact JSON
    Direction   string          `json:"direction"`   // inbound/outbound
    CreatedAt   time.Time       `json:"createdAt"`
    UpdatedAt   time.Time       `json:"updatedAt"`
}
```

### Project Structure
```
bot-portal/
├── cmd/
│   └── portal/
│       └── main.go              # Entry point
├── internal/
│   ├── api/
│   │   ├── router.go            # HTTP router setup
│   │   ├── agents.go            # Agent CRUD handlers
│   │   ├── channels.go          # Channel handlers
│   │   ├── messages.go          # Message log handlers
│   │   └── sse.go               # SSE streaming
│   ├── a2a/
│   │   ├── router.go            # A2A task routing (channel logic)
│   │   ├── client.go            # A2A client (POST /tasks, GET /tasks/{id}/stream)
│   │   ├── server.go            # A2A server (receive POST /tasks, serve SSE stream)
│   │   ├── types.go             # Google A2A types: AgentCard, Task, TaskMessage, etc.
│   │   └── discovery.go         # AgentCard fetching & caching
│   ├── docker/
│   │   └── manager.go           # Docker container lifecycle
│   ├── store/
│   │   ├── sqlite.go            # SQLite implementation
│   │   ├── agents.go            # Agent queries
│   │   ├── channels.go          # Channel queries
│   │   └── messages.go          # Message log queries
│   └── models/
│       └── models.go            # Domain models
├── web/                         # React frontend
│   ├── src/
│   │   ├── components/
│   │   ├── pages/
│   │   ├── hooks/
│   │   └── api/
│   ├── package.json
│   └── vite.config.ts
├── migrations/                  # SQL migrations
├── Dockerfile                   # Portal container
├── docker-compose.yml           # Dev environment
├── go.mod
├── go.sum
└── Makefile
```

## Alternatives Considered

| Option | Pros | Cons |
|--------|------|------|
| **Python (FastAPI) backend** | Faster prototyping, A2A Python SDK exists | Not team standard for services; weaker Docker SDK |
| **htmx/templ instead of React** | Simpler, no JS build step, Go-native | Limited interactivity for real-time log viewer; harder to build rich UIs |
| **PostgreSQL from day 1** | More scalable, better concurrency | Over-engineered for MVP; adds infra dependency |
| **WebSocket instead of SSE** | Bidirectional communication | A2A already uses SSE; we only need server→client for logs; more complex |
| **gRPC for internal comms** | Type-safe, streaming built-in | A2A standard is JSON-RPC/HTTP; adds unnecessary protocol translation |
| **Microservice per concern** | Better separation | Violates monolith-first principle; premature for MVP |

## Consequences

- **Positive**: Single deployable binary (Go + embedded React); simple ops; A2A-native communication; full message observability
- **Negative**: SQLite limits write concurrency (acceptable for MVP scale); monolith means all-or-nothing deploys
- **Risks**: A2A protocol is still evolving (RC v1.0) — may need spec updates; Docker socket access requires elevated permissions

## Implementation Notes
- **Affected services**: New greenfield project (bot-portal)
- **Migration plan**: N/A — greenfield
- **Rollback plan**: Standard git revert; no existing users or data

## MVP Scope (Phase 1)
1. Go backend with REST API + A2A router
2. SQLite storage
3. Docker container management
4. React frontend with agent list, config, and message viewer
5. SSE for real-time message streaming

## Future Phases
- Phase 2: Agent Card proxy/registry, push notifications, agent health monitoring
- Phase 3: PostgreSQL migration, authentication/RBAC, audit logging
- Phase 4: Agent marketplace, workflow orchestration, analytics dashboard
