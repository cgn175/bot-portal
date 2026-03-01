# CopilotKit Node.js Sidecar Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a Node.js sidecar service that implements the CopilotKit runtime GraphQL protocol, bridging the React frontend to our existing Go backend's chat completions API.

**Architecture:** The frontend sends GraphQL requests to a Node.js Express server (port 3001), which transforms them to OpenAI-compatible REST calls to our Go backend (port 8080), then streams responses back as GraphQL subscriptions. The sidecar is stateless and delegates all business logic to the existing Go API.

**Tech Stack:** Node.js, Express, @copilotkit/runtime, GraphQL, Server-Sent Events (SSE)

---

## Background

**Problem:** CopilotKit's React library expects a GraphQL endpoint at `runtimeUrl` that implements their specific schema (mutations like `GenerateCopilotResponse`, queries like `AvailableAgents`). Our Go backend implements OpenAI-compatible REST endpoints (`/api/chat/completions`, `/api/copilot/chat/completions`).

**Solution:** Create a thin Node.js service using `@copilotkit/runtime` that:
1. Accepts GraphQL requests from the CopilotKit React components
2. Uses the official CopilotKit runtime adapter to handle the protocol
3. Forwards LLM requests to our Go backend's `/api/copilot/chat/completions` endpoint
4. Returns responses in the expected GraphQL format

**Why Node.js instead of native Go GraphQL:**
- CopilotKit provides official runtime adapters for Node.js/Next.js
- Protocol is complex (GraphQL with SSE, tool calling, agent state)
- Implementing from scratch would take 10x longer and be brittle to updates
- The sidecar is stateless—all state lives in Go backend

---

## Task 1: Setup Node.js Sidecar Project

**Files:**
- Create: `sidecar/package.json`
- Create: `sidecar/tsconfig.json`
- Create: `sidecar/.gitignore`
- Create: `sidecar/README.md`

**Step 1: Create sidecar directory structure**

```bash
cd /Users/hoangta/projects/bot-portal
mkdir -p sidecar/src
```

**Step 2: Initialize npm project**

Create `sidecar/package.json`:

```json
{
  "name": "bot-portal-copilot-sidecar",
  "version": "1.0.0",
  "description": "CopilotKit runtime sidecar for Bot Portal",
  "type": "module",
  "main": "dist/index.js",
  "scripts": {
    "dev": "tsx watch src/index.ts",
    "build": "tsc",
    "start": "node dist/index.js",
    "type-check": "tsc --noEmit"
  },
  "dependencies": {
    "@copilotkit/runtime": "^1.52.1",
    "@copilotkit/sdk-js": "^1.52.1",
    "express": "^4.18.2",
    "cors": "^2.8.5"
  },
  "devDependencies": {
    "@types/express": "^4.17.21",
    "@types/cors": "^2.8.17",
    "@types/node": "^20.11.0",
    "tsx": "^4.7.0",
    "typescript": "^5.3.3"
  }
}
```

**Step 3: Create TypeScript config**

Create `sidecar/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ES2022",
    "moduleResolution": "node",
    "esModuleInterop": true,
    "strict": true,
    "skipLibCheck": true,
    "outDir": "./dist",
    "rootDir": "./src",
    "resolveJsonModule": true,
    "allowSyntheticDefaultImports": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist"]
}
```

**Step 4: Create .gitignore**

Create `sidecar/.gitignore`:

```
node_modules/
dist/
.env
*.log
.DS_Store
```

**Step 5: Create README**

Create `sidecar/README.md`:

```markdown
# Bot Portal CopilotKit Sidecar

Node.js service that bridges CopilotKit's GraphQL protocol to Bot Portal's Go backend.

## Setup

```bash
npm install
npm run dev  # Development with hot reload
```

## Architecture

Frontend (React) → GraphQL (port 3001) → Sidecar (Node.js) → REST (port 8080) → Go Backend
```

**Step 6: Install dependencies**

```bash
cd sidecar
npm install
```

Expected: All dependencies installed without errors

**Step 7: Commit**

```bash
cd ..
git add sidecar/
git commit -m "feat: initialize CopilotKit Node.js sidecar project"
```

---

## Task 2: Implement Basic Express Server with CopilotKit Runtime

**Files:**
- Create: `sidecar/src/index.ts`
- Create: `sidecar/src/config.ts`

**Step 1: Create config module**

Create `sidecar/src/config.ts`:

```typescript
export const config = {
  // Sidecar server port
  port: process.env.COPILOT_PORT || 3001,

  // Bot Portal Go backend base URL
  backendUrl: process.env.BACKEND_URL || 'http://localhost:8080',

  // CORS allowed origin (React dev server)
  corsOrigin: process.env.CORS_ORIGIN || 'http://localhost:5173',
};
```

**Step 2: Create basic Express server with CopilotKit runtime**

Create `sidecar/src/index.ts`:

```typescript
import express from 'express';
import cors from 'cors';
import { copilotRuntimeNodeHttpEndpoint } from '@copilotkit/runtime';
import { CopilotRuntime } from '@copilotkit/sdk-js';
import { config } from './config.js';

const app = express();

// CORS for React dev server
app.use(cors({
  origin: config.corsOrigin,
  credentials: true,
}));

// Health check
app.get('/health', (req, res) => {
  res.json({ status: 'ok', service: 'bot-portal-copilot-sidecar' });
});

// CopilotKit runtime endpoint
app.use('/copilot', (req, res, next) => {
  const serviceAdapter = {
    // For now, just return empty - we'll implement the adapter in next task
    async process() {
      return {
        messages: [],
        isStreamFinished: true
      };
    }
  };

  const runtime = new CopilotRuntime({
    actions: [],
  });

  return copilotRuntimeNodeHttpEndpoint({
    endpoint: '/copilot',
    runtime,
  })(req, res, next);
});

app.listen(config.port, () => {
  console.log(`CopilotKit sidecar running on http://localhost:${config.port}`);
  console.log(`Backend URL: ${config.backendUrl}`);
  console.log(`CORS origin: ${config.corsOrigin}`);
});
```

**Step 3: Test the server starts**

```bash
cd sidecar
npm run dev
```

Expected output:
```
CopilotKit sidecar running on http://localhost:3001
Backend URL: http://localhost:8080
CORS origin: http://localhost:5173
```

**Step 4: Test health endpoint**

In another terminal:
```bash
curl http://localhost:3001/health
```

Expected: `{"status":"ok","service":"bot-portal-copilot-sidecar"}`

**Step 5: Stop dev server** (Ctrl+C)

**Step 6: Commit**

```bash
cd ..
git add sidecar/src/
git commit -m "feat: add basic Express server with CopilotKit runtime stub"
```

---

## Task 3: Implement Backend Chat Adapter

**Files:**
- Create: `sidecar/src/backend-adapter.ts`
- Modify: `sidecar/src/index.ts`

**Step 1: Create backend adapter for chat completions**

Create `sidecar/src/backend-adapter.ts`:

```typescript
import { config } from './config.js';

export interface ChatMessage {
  role: 'system' | 'user' | 'assistant';
  content: string;
}

export interface ChatRequest {
  model?: string;
  messages: ChatMessage[];
  stream: boolean;
  tools?: any[];
  tool_choice?: any;
}

/**
 * BackendChatAdapter forwards chat requests to the Go backend's
 * /api/copilot/chat/completions endpoint and streams back responses.
 */
export class BackendChatAdapter {
  private backendUrl: string;

  constructor(backendUrl: string = config.backendUrl) {
    this.backendUrl = backendUrl;
  }

  /**
   * Send chat completion request to Go backend.
   * Returns async generator that yields SSE chunks.
   */
  async *streamChatCompletion(request: ChatRequest): AsyncGenerator<string> {
    const url = `${this.backendUrl}/api/copilot/chat/completions`;

    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        ...request,
        stream: true, // Always stream
      }),
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`Backend error: ${response.status} - ${errorText}`);
    }

    if (!response.body) {
      throw new Error('No response body from backend');
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';

    try {
      while (true) {
        const { done, value } = await reader.read();

        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          if (line.trim() === '') continue;
          if (line.startsWith('data: ')) {
            const data = line.slice(6);
            if (data === '[DONE]') {
              return;
            }
            yield data;
          }
        }
      }
    } finally {
      reader.releaseLock();
    }
  }

  /**
   * Get available models from backend.
   */
  async getAvailableModels(): Promise<any[]> {
    const url = `${this.backendUrl}/api/copilot/info`;
    const response = await fetch(url);

    if (!response.ok) {
      throw new Error(`Failed to fetch models: ${response.status}`);
    }

    const data = await response.json();
    return data.models || [];
  }
}
```

**Step 2: Integrate adapter into runtime**

Modify `sidecar/src/index.ts` to use the adapter:

```typescript
import express from 'express';
import cors from 'cors';
import { copilotRuntimeNodeHttpEndpoint } from '@copilotkit/runtime';
import { CopilotRuntime } from '@copilotkit/sdk-js';
import { config } from './config.js';
import { BackendChatAdapter } from './backend-adapter.js';

const app = express();

app.use(cors({
  origin: config.corsOrigin,
  credentials: true,
}));

app.get('/health', (req, res) => {
  res.json({ status: 'ok', service: 'bot-portal-copilot-sidecar' });
});

// Create backend adapter
const backendAdapter = new BackendChatAdapter();

app.use('/copilot', async (req, res, next) => {
  const runtime = new CopilotRuntime({
    actions: [],
  });

  // TODO: Wire up the adapter to CopilotRuntime's service adapter interface
  // This will be completed in Task 4

  return copilotRuntimeNodeHttpEndpoint({
    endpoint: '/copilot',
    runtime,
  })(req, res, next);
});

app.listen(config.port, () => {
  console.log(`CopilotKit sidecar running on http://localhost:${config.port}`);
  console.log(`Backend URL: ${config.backendUrl}`);
});
```

**Step 3: Verify compilation**

```bash
cd sidecar
npm run type-check
```

Expected: No TypeScript errors

**Step 4: Commit**

```bash
cd ..
git add sidecar/src/
git commit -m "feat: implement backend chat adapter for Go API integration"
```

---

## Task 4: Wire CopilotRuntime to Backend Adapter

**Files:**
- Modify: `sidecar/src/index.ts`
- Create: `sidecar/src/runtime-adapter.ts`

**Step 1: Study CopilotKit's service adapter interface**

Review the `@copilotkit/sdk-js` documentation and type definitions to understand how to implement a custom LLM service adapter. The key interface is `CopilotRuntimeServiceAdapter`.

**Step 2: Create runtime adapter bridge**

Create `sidecar/src/runtime-adapter.ts`:

```typescript
import { BackendChatAdapter } from './backend-adapter.js';

/**
 * Adapter that bridges CopilotKit's runtime to our Go backend.
 * Implements the CopilotRuntime service adapter interface.
 */
export function createBackendRuntimeAdapter(backendAdapter: BackendChatAdapter) {
  return {
    async *streamChatCompletion(params: {
      messages: any[];
      tools?: any[];
      model?: string;
    }): AsyncGenerator<any> {
      // Transform CopilotKit messages to our backend format
      const backendMessages = params.messages.map((msg: any) => ({
        role: msg.role,
        content: msg.content || msg.textContent || '',
      }));

      // Stream from backend
      for await (const chunk of backendAdapter.streamChatCompletion({
        model: params.model,
        messages: backendMessages,
        stream: true,
        tools: params.tools,
      })) {
        try {
          const parsed = JSON.parse(chunk);

          // Transform OpenAI streaming format to CopilotKit format
          if (parsed.choices && parsed.choices[0]?.delta?.content) {
            yield {
              type: 'text',
              content: parsed.choices[0].delta.content,
            };
          }

          // Handle tool calls if present
          if (parsed.choices && parsed.choices[0]?.delta?.tool_calls) {
            for (const toolCall of parsed.choices[0].delta.tool_calls) {
              yield {
                type: 'tool_call',
                toolCall,
              };
            }
          }
        } catch (e) {
          console.error('Failed to parse SSE chunk:', chunk, e);
        }
      }
    },
  };
}
```

**Step 3: Integrate runtime adapter**

Modify `sidecar/src/index.ts`:

```typescript
import express from 'express';
import cors from 'cors';
import { copilotRuntimeNodeHttpEndpoint } from '@copilotkit/runtime';
import { CopilotRuntime } from '@copilotkit/sdk-js';
import { config } from './config.js';
import { BackendChatAdapter } from './backend-adapter.js';
import { createBackendRuntimeAdapter } from './runtime-adapter.js';

const app = express();

app.use(cors({
  origin: config.corsOrigin,
  credentials: true,
}));

app.get('/health', (req, res) => {
  res.json({ status: 'ok', service: 'bot-portal-copilot-sidecar' });
});

const backendAdapter = new BackendChatAdapter();
const serviceAdapter = createBackendRuntimeAdapter(backendAdapter);

app.use('/copilot', async (req, res, next) => {
  const runtime = new CopilotRuntime({
    actions: [],
    serviceAdapter,
  });

  return copilotRuntimeNodeHttpEndpoint({
    endpoint: '/copilot',
    runtime,
  })(req, res, next);
});

app.listen(config.port, () => {
  console.log(`CopilotKit sidecar running on http://localhost:${config.port}`);
  console.log(`Backend URL: ${config.backendUrl}`);
});
```

**Step 4: Test type checking**

```bash
cd sidecar
npm run type-check
```

Expected: No errors (or document any type mismatches to fix)

**Step 5: Commit**

```bash
cd ..
git add sidecar/src/
git commit -m "feat: wire CopilotRuntime to backend adapter"
```

---

## Task 5: Update Frontend to Use Sidecar

**Files:**
- Modify: `web/src/copilot/CopilotProvider.tsx`
- Modify: `sidecar/README.md`

**Step 1: Update CopilotProvider to point to sidecar**

Modify `web/src/copilot/CopilotProvider.tsx`:

```typescript
import React from 'react';
import { CopilotKit } from '@copilotkit/react-core';
import '@copilotkit/react-ui/styles.css';

interface CopilotProviderProps {
  children: React.ReactNode;
}

/**
 * CopilotProvider wraps the app with CopilotKit configured for self-hosted runtime.
 *
 * Connects to Node.js sidecar (port 3001) which bridges to Go backend.
 */
export function CopilotProvider({ children }: CopilotProviderProps) {
  const runtimeUrl = import.meta.env.VITE_COPILOT_RUNTIME_URL || 'http://localhost:3001/copilot';

  return (
    <CopilotKit
      runtimeUrl={runtimeUrl}
      showDevConsole={import.meta.env.DEV}
    >
      {children}
    </CopilotKit>
  );
}
```

**Step 2: Add environment variable documentation**

Modify `sidecar/README.md` to add environment variables section:

```markdown
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
```

**Step 3: Add .env.example for frontend**

Create `web/.env.example`:

```bash
# CopilotKit Runtime URL (Node.js sidecar)
VITE_COPILOT_RUNTIME_URL=http://localhost:3001/copilot
```

**Step 4: Commit**

```bash
git add web/src/copilot/CopilotProvider.tsx web/.env.example sidecar/README.md
git commit -m "feat: configure frontend to use Node.js sidecar runtime"
```

---

## Task 6: Add Startup Scripts and Documentation

**Files:**
- Modify: `Makefile`
- Modify: `README.md`
- Create: `sidecar/Dockerfile`
- Modify: `docker-compose.yml`

**Step 1: Add sidecar targets to Makefile**

Modify `Makefile` to add:

```makefile
.PHONY: sidecar-install sidecar-dev sidecar-build dev-all

sidecar-install:
	cd sidecar && npm install

sidecar-dev:
	cd sidecar && npm run dev

sidecar-build:
	cd sidecar && npm run build

# Run all services in development
dev-all:
	@echo "Starting Go backend..."
	@make run &
	@sleep 2
	@echo "Starting Node.js sidecar..."
	@cd sidecar && npm run dev &
	@sleep 2
	@echo "Starting React frontend..."
	@cd web && npm run dev
```

**Step 2: Create Dockerfile for sidecar**

Create `sidecar/Dockerfile`:

```dockerfile
FROM node:20-alpine

WORKDIR /app

COPY package*.json ./
RUN npm ci --production

COPY tsconfig.json ./
COPY src ./src

RUN npm run build

EXPOSE 3001

CMD ["npm", "start"]
```

**Step 3: Add sidecar to docker-compose**

Modify `docker-compose.yml` to add sidecar service:

```yaml
services:
  # ... existing services ...

  copilot-sidecar:
    build:
      context: ./sidecar
      dockerfile: Dockerfile
    ports:
      - "3001:3001"
    environment:
      - COPILOT_PORT=3001
      - BACKEND_URL=http://portal:8080
      - CORS_ORIGIN=http://localhost:5173
    depends_on:
      - portal
    restart: unless-stopped
```

**Step 4: Update main README**

Add to `README.md` (or modify existing development section):

```markdown
## Development with CopilotKit

The project uses a Node.js sidecar to bridge CopilotKit's GraphQL protocol to the Go backend.

### Start all services:

```bash
# Terminal 1: Go backend
make run

# Terminal 2: Node.js sidecar
cd sidecar && npm run dev

# Terminal 3: React frontend
cd web && npm run dev
```

Or use the combined command:

```bash
make dev-all
```

### Architecture

```
React Frontend (:5173) → CopilotKit → Sidecar (:3001) → Go Backend (:8080) → LLM Providers
```
```

**Step 5: Commit**

```bash
git add Makefile README.md sidecar/Dockerfile docker-compose.yml
git commit -m "chore: add sidecar build scripts and docker config"
```

---

## Task 7: Testing and Verification

**Files:**
- Create: `sidecar/test-sidecar.sh`
- Modify: `CLAUDE.md`

**Step 1: Create test script**

Create `sidecar/test-sidecar.sh`:

```bash
#!/bin/bash
set -e

echo "Testing CopilotKit sidecar..."

# Test 1: Health check
echo "1. Health check..."
HEALTH=$(curl -s http://localhost:3001/health)
if [[ $HEALTH == *"ok"* ]]; then
  echo "✓ Health check passed"
else
  echo "✗ Health check failed"
  exit 1
fi

# Test 2: GraphQL endpoint exists
echo "2. GraphQL endpoint check..."
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:3001/copilot)
if [[ $STATUS == "200" ]] || [[ $STATUS == "400" ]]; then
  echo "✓ GraphQL endpoint responding"
else
  echo "✗ GraphQL endpoint not responding (HTTP $STATUS)"
  exit 1
fi

# Test 3: Backend connectivity
echo "3. Backend connectivity..."
cd ../
BACKEND_INFO=$(curl -s http://localhost:8080/api/copilot/info)
if [[ $BACKEND_INFO == *"models"* ]]; then
  echo "✓ Backend reachable"
else
  echo "✗ Backend not reachable"
  exit 1
fi

echo ""
echo "All tests passed! ✓"
echo "Frontend should now work at http://localhost:5173"
```

Make it executable:

```bash
chmod +x sidecar/test-sidecar.sh
```

**Step 2: Update CLAUDE.md with sidecar info**

Add to `CLAUDE.md`:

```markdown
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
```

**Step 3: Run manual test**

Start services:

```bash
# Terminal 1
make run

# Terminal 2
cd sidecar && npm run dev

# Wait for both to start, then test
./sidecar/test-sidecar.sh
```

Expected: All tests pass

**Step 4: Commit**

```bash
git add sidecar/test-sidecar.sh CLAUDE.md
git commit -m "test: add sidecar verification script and documentation"
```

---

## Task 8: Frontend Integration Test

**Files:**
- Create: `docs/plans/copilotkit-sidecar-testing.md`

**Step 1: Document manual testing steps**

Create `docs/plans/copilotkit-sidecar-testing.md`:

```markdown
# CopilotKit Sidecar Integration Testing

## Prerequisites

1. Go backend running (port 8080) with at least one model configured
2. Node.js sidecar running (port 3001)
3. React frontend running (port 5173)

## Test Procedure

### 1. Open the App

Navigate to http://localhost:5173

**Expected:**
- App loads without errors
- CopilotKit popup appears in bottom-right corner (or chat icon)
- No errors in browser console

### 2. Open CopilotKit Chat

Click the chat icon/popup

**Expected:**
- Chat window opens
- Shows initial message: "Hi! I can help you manage agents, models, and auth configs. What would you like to do?"

### 3. Test Context Exposure

Type: "What agents do I have?"

**Expected:**
- Request sent to http://localhost:3001/copilot (check Network tab)
- Response streams back (SSE or GraphQL subscription)
- Assistant responds with current agents list
- If no agents: "You don't have any agents configured yet."

### 4. Test Actions

Type: "Create a test agent named demo-agent"

**Expected:**
- CopilotKit calls the `createAgent` action
- Agent appears in the agents list on the main page
- Success message in chat

### 5. Test Error Handling

Stop the Go backend, then type a message

**Expected:**
- Sidecar returns error to frontend
- Chat shows user-friendly error message
- Error appears in sidecar console logs

### 6. Test Streaming

Ask a complex question: "Explain how agents work in Bot Portal"

**Expected:**
- Response streams word-by-word
- No long delays before text appears
- Complete response renders properly

## Debugging

### Check Sidecar Logs

```bash
cd sidecar
npm run dev
# Watch for GraphQL requests and backend calls
```

### Check Go Backend Logs

```bash
make run
# Watch for /api/copilot/chat/completions requests
```

### Browser DevTools

- Network tab: Look for requests to localhost:3001/copilot
- Console: Check for CopilotKit errors
- React DevTools: Verify CopilotProvider is rendering

## Common Issues

| Issue | Cause | Fix |
|-------|-------|-----|
| 404 on /api/copilot | Frontend pointing to wrong URL | Check `VITE_COPILOT_RUNTIME_URL` |
| CORS errors | Sidecar CORS misconfigured | Set `CORS_ORIGIN=http://localhost:5173` |
| No streaming | Backend not streaming | Verify Go `/api/copilot/chat/completions` returns SSE |
| Actions not working | Actions not registered | Check `CopilotActions` component is rendered |
```

**Step 2: Perform manual test**

Follow the testing procedure documented above.

**Step 3: Document any issues found**

If issues are found, create follow-up tasks in beads:

```bash
bd create "Fix: [specific issue found during testing]" -t bug -p 1
```

**Step 4: Commit testing documentation**

```bash
git add docs/plans/copilotkit-sidecar-testing.md
git commit -m "docs: add CopilotKit sidecar integration testing guide"
```

---

## Summary

This plan implements a Node.js sidecar that:

1. ✅ Bridges CopilotKit's GraphQL protocol to our Go backend's REST API
2. ✅ Reuses existing infrastructure (chat completions, models, auth configs)
3. ✅ Is stateless and delegates all business logic to Go
4. ✅ Supports streaming responses
5. ✅ Works with existing CopilotKit React components
6. ✅ Is containerized and can run in production

**Key Files Created:**
- `sidecar/` - Complete Node.js service
- `sidecar/src/index.ts` - Express server with CopilotRuntime
- `sidecar/src/backend-adapter.ts` - HTTP client for Go backend
- `sidecar/src/runtime-adapter.ts` - Bridge to CopilotKit SDK

**Testing:**
- Run `./sidecar/test-sidecar.sh` for automated checks
- Follow `docs/plans/copilotkit-sidecar-testing.md` for frontend testing

**Next Steps:**
- Deploy sidecar alongside Go backend
- Monitor performance and error rates
- Consider adding caching layer if needed
