# Bot Portal

A web portal for managing multiple AI agents running as Docker containers. Serves as an A2A (Agent-to-Agent) message router with channel-based communication, message logging, and agent lifecycle management.

## Features

- **Agent Management**: Register, start, stop, restart, and delete AI agents
- **A2A Message Router**: Route messages between agents using Google A2A protocol
- **Channel-based Communication**: Direct channels (agent1::agent2) and broadcast (general)
- **Message Logging**: Persistent task logs with full conversation history
- **Docker Integration**: Automatic container lifecycle management
- **Bearer Token Auth**: Secure agent-to-agent authentication
- **REST API**: Full HTTP API for portal management
- **React Frontend**: Web dashboard for monitoring and control

## Architecture

```
┌─────────────┐
│   Frontend  │ (React)
└──────┬──────┘
       │ HTTP
┌──────▼──────────────────┐
│   Bot Portal (Go)       │
│  - REST API             │
│  - A2A Router           │
│  - Docker Manager       │
└──────┬──────────────────┘
       │
┌──────▼──────┐  ┌─────────┐
│   SQLite    │  │  Docker │
│  Database   │  │  Engine │
└─────────────┘  └────┬────┘
                      │
        ┌─────────────┼─────────────┐
        │             │             │
   ┌────▼───┐   ┌────▼───┐   ┌────▼───┐
   │ Agent1 │   │ Agent2 │   │ Agent3 │
   └────────┘   └────────┘   └────────┘
```

## Quick Start

### Prerequisites

- Go 1.21+
- Docker
- Node.js 18+ (for frontend)

### Installation

```bash
# Clone repository
git clone https://github.com/zeroclaw/bot-portal.git
cd bot-portal

# Build backend
make build

# Run portal
./bin/bot-portal
```

The portal will start on `http://localhost:8080`

### Docker Compose

```bash
docker-compose up -d
```

## API Endpoints

### Agent Management

```bash
# List agents
GET /api/agents

# Create agent
POST /api/agents
{
  "id": "agent1",
  "name": "My Agent",
  "image": "my-agent:latest",
  "endpoint": "http://localhost:8081"
}

# Get agent
GET /api/agents/{id}

# Update agent
PUT /api/agents/{id}

# Delete agent
DELETE /api/agents/{id}

# Start agent
GET /api/agents/{id}?action=start

# Stop agent
GET /api/agents/{id}?action=stop

# Restart agent
GET /api/agents/{id}?action=restart
```

### A2A Protocol (Google A2A)

```bash
# Agent card
GET /.well-known/agent.json

# Create task
POST /tasks
Authorization: Bearer {agent-token}
{
  "channel_id": "agent1::agent2",
  "sender_id": "agent1",
  "message": {
    "role": "user",
    "content": "Hello"
  }
}

# Get task
GET /tasks/{id}
Authorization: Bearer {agent-token}

# Stream task updates (SSE)
GET /tasks/{id}/stream
Authorization: Bearer {agent-token}

# Cancel task
POST /tasks/{id}?cancel
Authorization: Bearer {agent-token}
```

### Channels

```bash
# List channels
GET /api/channels

# Create channel
POST /api/channels
{
  "id": "agent1::agent2",
  "members": ["agent1", "agent2"]
}

# Get channel
GET /api/channels/{id}
```

### Messages

```bash
# Stream messages (SSE)
GET /api/messages/stream?channel_id={id}
```

## Configuration

Environment variables:

```bash
# Database path
DB_PATH=./bot-portal.db

# Server port
PORT=8080

# Docker network
DOCKER_NETWORK=bot-portal
```

## Development

### Run Tests

```bash
# All tests
make test

# Unit tests only
go test ./internal/...

# Integration tests
cd test
go test -v ./integration_test.go

# With mock agents
cd test
docker-compose -f docker-compose.test.yml up -d
go test -v ./integration_test.go
docker-compose -f docker-compose.test.yml down
```

### Project Structure

```
bot-portal/
├── cmd/portal/          # Main application entry point
├── internal/
│   ├── api/            # REST API handlers
│   ├── a2a/            # A2A protocol router
│   ├── docker/         # Docker container manager
│   ├── store/          # Database layer (SQLite)
│   └── models/         # Data models
├── web/                # React frontend
├── test/               # Integration tests
│   └── mock-agent/     # Mock agent for testing
├── migrations/         # Database migrations
└── Makefile
```

## Agent Requirements

Agents must implement the Google A2A protocol:

1. **Agent Card**: `GET /.well-known/agent.json`
2. **Task Endpoint**: `POST /tasks` (accepts task requests)
3. **Health Check**: `GET /health` (optional)

### Example Agent

See `test/mock-agent/mock-agent.go` for a minimal implementation.

## Docker Network

Agents run in the `bot-portal` bridge network for inter-agent communication. The portal automatically creates this network on startup.

## Security

- **Bearer Token Authentication**: All A2A endpoints require valid agent tokens
- **Token Generation**: Cryptographically secure 32-byte random tokens
- **CORS**: Configurable for frontend development

## Database

Uses SQLite with the pure Go driver (`modernc.org/sqlite`) for:
- Agent registry
- Channel management
- Task logs and message history

Database indexes on:
- `task_logs.channel_id`
- `task_logs.sender_id`
- `task_logs.created_at`

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

MIT

## Support

For issues and questions, please open a GitHub issue.
