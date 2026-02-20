# ADR-001: Bot Portal Architecture

## Status
Proposed

## Context
We need a web-based portal to manage multiple zeroclaw AI agents. Each agent runs as a Docker container and communicates via the Google A2A (Agent2Agent) protocol. The portal serves as:
1. A **messaging router/hub** for agent-to-agent communication
2. A **management UI** for agent lifecycle (start/stop/configure)
3. A **log viewer** for all inter-agent messages

### Key Requirements
- Agents are identified by unique IDs
- Communication channels: `agent1-id::agent2-id` (direct) or `general` (broadcast)
- All A2A messages must be logged and viewable
- Agent configuration (name, skills, env vars) managed through the portal
- Portal acts as the A2A registry (agents discover each other through it)

## Decision

### Tech Stack

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| Backend API | **Go** | Team standard for microservices; excellent Docker SDK; strong concurrency for message routing |
| Frontend | **React + TypeScript + Vite** | Rich interactivity needed for real-time log viewer, agent management UI |
| Database | **SQLite** (MVP) → **PostgreSQL** (scale) | Simple start, no infra overhead; easy migration path |
| Real-time | **Server-Sent Events (SSE)** | A2A already uses SSE for streaming; consistent pattern; simpler than WebSocket for unidirectional updates |
| Container Mgmt | **Docker Engine API** (Go SDK) | Direct container lifecycle control |
| A2A Protocol | **JSON-RPC 2.0 over HTTP** | Standard A2A binding; portal is both A2A client and server |

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
The portal acts as **both A2A Client and A2A Server**:
- **As A2A Server**: Receives messages from agents via `POST /a2a` (JSON-RPC endpoint)
- **As A2A Client**: Forwards messages to target agents via their A2A endpoints
- **Channel system**: Routes messages based on channel format:
  - `agentA-id::agentB-id` — direct channel, routed to specific agent
  - `general` — broadcast to all registered agents
- **Message logging**: Every message passing through the router is persisted to DB with metadata (timestamp, channel, sender, receiver, content)

#### 2. Agent Registry & Discovery
- Portal maintains a registry of all agents and their Agent Cards
- Serves `/.well-known/agent-card.json` for each registered agent (proxy)
- Agents register with the portal on startup (or portal discovers them via Docker labels)
- Portal exposes `GET /api/agents` — returns all registered agent cards

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
| `/api/channels` | GET | List all channels |
| `/api/channels/:id/messages` | GET | Get channel message history |
| `/api/messages/stream` | GET (SSE) | Real-time message stream |
| `/a2a` | POST | A2A JSON-RPC endpoint (agents send messages here) |

#### 5. Frontend (React)
- **Dashboard**: Overview of all agents, their status (running/stopped/error)
- **Agent Detail**: Config editor, container logs, agent card viewer
- **Message Viewer**: Real-time log of all A2A messages, filterable by channel
- **Channel View**: View specific channel conversations

### Data Models

#### Agent
```go
type Agent struct {
    ID          string            `json:"id"`
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Image       string            `json:"image"`       // Docker image
    Status      string            `json:"status"`      // running, stopped, error
    ContainerID string            `json:"containerId"`
    Endpoint    string            `json:"endpoint"`    // A2A endpoint URL
    Config      map[string]string `json:"config"`      // env vars / settings
    Skills      []AgentSkill      `json:"skills"`
    CreatedAt   time.Time         `json:"createdAt"`
    UpdatedAt   time.Time         `json:"updatedAt"`
}
```

#### Channel
```go
type Channel struct {
    ID        string    `json:"id"`        // "agentA::agentB" or "general"
    Members   []string  `json:"members"`   // agent IDs
    CreatedAt time.Time `json:"createdAt"`
}
```

#### MessageLog
```go
type MessageLog struct {
    ID        string    `json:"id"`
    ChannelID string    `json:"channelId"`
    SenderID  string    `json:"senderId"`
    ReceiverID string   `json:"receiverId"`
    TaskID    string    `json:"taskId"`
    Method    string    `json:"method"`     // SendMessage, GetTask, etc.
    Content   json.RawMessage `json:"content"` // full A2A message payload
    Direction string    `json:"direction"`  // inbound/outbound
    Timestamp time.Time `json:"timestamp"`
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
│   │   ├── router.go            # A2A message routing logic
│   │   ├── client.go            # A2A client (send to agents)
│   │   ├── server.go            # A2A server (receive from agents)
│   │   └── types.go             # A2A protocol types
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
