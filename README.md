# Bot Portal

A web portal for managing multiple AI agents running as Docker or Podman containers. Serves as an A2A (Agent-to-Agent) message router with channel-based communication, message logging, and agent lifecycle management.

## Features

- **Agent Management**: Register, start, stop, restart, and delete AI agents
- **A2A Message Router**: Route messages between agents using Google A2A protocol
- **Channel-based Communication**: Direct channels (agent1::agent2) and broadcast (general)
- **Message Logging**: Persistent task logs with full conversation history
- **Docker & Podman Integration**: Automatic container lifecycle management with support for Docker Desktop and Podman Machine
- **Bearer Token Auth**: Secure agent-to-agent authentication
- **Central Model Store**: Manage shared AI model configurations across agents
- **Authentication Store**: Secure API key and credential management encrypted at rest
- **25+ AI Providers**: Pre-configured support for OpenAI, Anthropic, Kimi, DeepSeek, GLM, MiniMax, Qwen, and more
- **Auto Model Discovery**: Automatically fetch available models from provider endpoints
- **Test Chat**: Interactive chat interface to test models directly in the portal
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
- Docker or Podman
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

### Running with Docker (Recommended)

To ensure the portal can correctly communicate with agents in their isolated networks (for "Test Connection" and task routing), it is recommended to run the portal itself in a container within the same network.

```bash
docker-compose up -d
```

This will start the portal and its required infrastructure. The portal will be accessible at `http://localhost:8080`.

## Supported AI Providers

Bot Portal includes pre-configured support for 25+ AI providers with correct endpoints, headers, and authentication:

### Popular Providers
| Provider | ID | Endpoint | Auth Type |
|----------|-----|----------|-----------|
| OpenAI | `openai` | https://api.openai.com/v1 | API Key |
| Anthropic | `anthropic` | https://api.anthropic.com/v1 | API Key |
| GitHub Copilot | `github_copilot` | https://api.githubcopilot.com | OAuth |
| Google Gemini | `gemini` | https://generativelanguage.googleapis.com/v1beta | API Key |

### Chinese Providers
| Provider | ID | Endpoint | Notes |
|----------|-----|----------|-------|
| Kimi (Moonshot) | `kimi` | https://api.moonshot.cn/v1 | User-Agent header auto-applied |
| Kimi Code | `kimi-code` | https://api.kimi.com/coding/v1 | Coding-optimized models |
| DeepSeek | `deepseek` | https://api.deepseek.com | - |
| GLM (Zhipu) | `glm` | https://open.bigmodel.cn/api/paas/v4 | - |
| MiniMax | `minimax` | https://api.minimaxi.com/v1 | Global endpoint |
| Qwen | `qwen` | https://dashscope.aliyuncs.com/compatible-mode/v1 | Alibaba Dashscope |
| Qwen Code | `qwen-code` | https://chat.qwen.ai/api | OAuth support |
| Baidu Qianfan | `qianfan` | https://aip.baidubce.com | - |

### Open Source & Inference
| Provider | ID | Endpoint |
|----------|-----|----------|
| Groq | `groq` | https://api.groq.com/openai/v1 |
| Together AI | `together` | https://api.together.xyz/v1 |
| Fireworks | `fireworks` | https://api.fireworks.ai/inference/v1 |
| Mistral | `mistral` | https://api.mistral.ai/v1 |
| Ollama | `ollama` | http://localhost:11434/v1 |
| LM Studio | `lmstudio` | http://localhost:1234/v1 |
| OpenRouter | `openrouter` | https://openrouter.ai/api/v1 |

### Enterprise
| Provider | ID | Endpoint |
|----------|-----|----------|
| xAI (Grok) | `xai` | https://api.x.ai/v1 |
| Perplexity | `perplexity` | https://api.perplexity.ai |
| Cohere | `cohere` | https://api.cohere.com/compatibility/v1 |
| Cloudflare AI | `cloudflare` | https://gateway.ai.cloudflare.com/v1 |

### Custom Providers
For any OpenAI-compatible endpoint not listed, use the **Custom Provider** option and specify your own endpoint URL.

### Model Discovery
When you add an auth config with a provider:
1. The portal automatically queries the provider's `/models` endpoint
2. Available models are saved to the database
3. Models are automatically deleted when the auth config is removed

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

### Models

```bash
# List models
GET /api/models

# Create model
POST /api/models
{
  "id": "gpt-4",
  "name": "GPT-4",
  "provider": "openai",
  "model_identifier": "gpt-4o",
  "endpoint_url": "https://api.openai.com/v1",
  "default_params": "{\"temperature\": 0.7}"
}

# Get, Update, Delete model
GET /api/models/{id}
PUT /api/models/{id}
DELETE /api/models/{id}
```

### Auth Configs

```bash
# List auth configs
GET /api/auth-configs

# Create auth config
POST /api/auth-configs
{
  "id": "openai-key",
  "name": "OpenAI Primary Key",
  "auth_type": "bearer",
  "credentials": "{\"api_key\": \"sk-...\"}"
}

# Get, Update, Delete auth config
GET /api/auth-configs/{id}
PUT /api/auth-configs/{id}
DELETE /api/auth-configs/{id}
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

# Encryption Key (required for Auth Configs, must be exactly 32 bytes)
ENCRYPTION_KEY=my-secure-32-byte-encryption-key-
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
