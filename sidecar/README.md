# Bot Portal CopilotKit Sidecar

Node.js service that bridges CopilotKit's GraphQL protocol to Bot Portal's Go backend.

## Setup

```bash
npm install
npm run dev  # Development with hot reload
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `COPILOT_PORT` | `3001` | Port for sidecar server |
| `BACKEND_URL` | `http://localhost:8080` | Bot Portal Go backend URL |
| `CORS_ORIGIN` | `http://localhost:5173` | React dev server URL |

## Architecture

```
Frontend (React :5173) → GraphQL → Sidecar (Node.js :3001) → REST → Go Backend (:8080)
```

The sidecar is stateless—all business logic lives in the Go backend.
