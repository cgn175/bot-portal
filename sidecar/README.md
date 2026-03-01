# Bot Portal CopilotKit Sidecar

Node.js service that bridges CopilotKit's GraphQL protocol to Bot Portal's Go backend.

## Setup

```bash
npm install
npm run dev  # Development with hot reload
```

## Architecture

Frontend (React) → GraphQL (port 3001) → Sidecar (Node.js) → REST (port 8080) → Go Backend
