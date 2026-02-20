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
Zeroclaw agents use a **custom A2A protocol** (not Google's standard A2A spec):
- **Transport**: HTTP/2 + SSE for bidirectional communication
- **Send**: `POST /a2a/send` — JSON body with `A2AMessage` envelope
- **Stream**: `GET /a2a/stream/:session_id` — SSE response stream
- **Health**: `GET /a2a/health` — public health check
- **Pairing**: `POST /a2a/pair/request` + `POST /a2a/pair/confirm`
- **Auth**: Bearer token per peer (encrypted at rest)
- **Port**: 9000 (configurable via `listen_port`)
- **Config**: TOML at `~/.zeroclaw/config.toml` under `[channels_config.a2a]`

#### A2AMessage Format
```json
{
  "id": "uuid-v4",
  "session_id": "conversation-thread-id",
  "sender_id": "peer-identity",
  "recipient_id": "target-peer",
  "content": "message text",
  "timestamp": 1700000000,
  "reply_to": "optional-parent-message-id"
}
```

#### Agent Config Structure (TOML)
```toml
[channels_config.a2a]
enabled = true
listen_port = 9000
discovery_mode = "static"
allowed_peer_ids = ["*"]

[[channels_config.a2a.peers]]
id = "agent-alpha"
endpoint = "https://192.168.1.100:9000"
bearer_token = "encrypted:..."
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
| A2A Protocol | **ZeroClaw A2A** (HTTP/2 + SSE) | Native zeroclaw protocol; portal is both A2A peer and router |

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
The portal acts as a **zeroclaw A2A peer and message router**:
- **Receives**: Agents send messages to portal via `POST /a2a/send` (portal listens on its own A2A port)
- **Forwards**: Portal forwards messages to target agents via `POST /a2a/send` on their endpoints
- **Streams**: Portal connects to each agent's `GET /a2a/stream/:session_id` for response streaming
- **Channel system**: Routes messages based on channel format:
  - `agentA-id::agentB-id` — direct channel, routed to specific agent
  - `general` — broadcast to all registered agents
- **Message logging**: Every A2AMessage passing through is persisted with metadata (id, session_id, sender_id, recipient_id, content, timestamp, channel)
- **Health monitoring**: Polls each agent's `GET /a2a/health` endpoint

#### 2. Agent Registry & Discovery
- Portal maintains a registry of all agents (peer configs)
- When an agent is registered, portal auto-generates bearer tokens and configures the agent's `config.toml` `[channels_config.a2a]` section
- Each agent's peer list includes the portal + other agents it should communicate with
- Portal exposes `GET /api/agents` — returns all registered agents and their status

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
| `/api/messages/stream` | GET (SSE) | Real-time message stream |
| `/a2a/send` | POST | ZeroClaw A2A receive endpoint (agents send messages here) |
| `/a2a/stream/:session_id` | GET (SSE) | A2A response stream |
| `/a2a/health` | GET | A2A health check |

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
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Image       string            `json:"image"`       // Docker image
    Status      string            `json:"status"`      // running, stopped, error
    ContainerID string            `json:"containerId"`
    Endpoint    string            `json:"endpoint"`    // A2A endpoint (e.g. http://container:9000)
    ListenPort  int               `json:"listenPort"`  // A2A listen port (default 9000)
    BearerToken string            `json:"-"`           // never exposed in API responses
    Config      map[string]string `json:"config"`      // env vars / settings
    CreatedAt   time.Time         `json:"createdAt"`
    UpdatedAt   time.Time         `json:"updatedAt"`
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

#### MessageLog (mirrors zeroclaw A2AMessage)
```go
type MessageLog struct {
    ID          string    `json:"id"`          // A2AMessage.id (UUID)
    SessionID   string    `json:"sessionId"`   // A2AMessage.session_id
    ChannelID   string    `json:"channelId"`   // derived: "sender::recipient" or "general"
    SenderID    string    `json:"senderId"`    // A2AMessage.sender_id
    RecipientID string    `json:"recipientId"` // A2AMessage.recipient_id
    Content     string    `json:"content"`     // A2AMessage.content
    ReplyTo     *string   `json:"replyTo"`     // A2AMessage.reply_to
    Direction   string    `json:"direction"`   // inbound/outbound
    Timestamp   time.Time `json:"timestamp"`   // from A2AMessage.timestamp
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
│   │   ├── router.go            # A2A message routing (channel logic)
│   │   ├── client.go            # A2A client (POST /a2a/send to agents)
│   │   ├── server.go            # A2A server (receive POST /a2a/send, SSE stream)
│   │   ├── types.go             # ZeroClaw A2AMessage, A2APeer, A2AConfig types
│   │   └── health.go            # Agent health check polling
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
