import express from 'express';
import cors from 'cors';
import { copilotRuntimeNodeHttpEndpoint } from '@copilotkit/runtime';
import { CopilotRuntime } from '@copilotkit/runtime';
import { config } from './config.js';
import { BackendChatAdapter } from './backend-adapter.js';
import { BackendRuntimeAdapter } from './runtime-adapter.js';

const app = express();

app.use(express.json());

app.use(cors({
  origin: config.corsOrigin,
  credentials: true,
}));

app.get('/health', (req, res) => {
  res.json({ status: 'ok', service: 'bot-portal-copilot-sidecar' });
});

// Create backend adapter and wire to runtime
const backendAdapter = new BackendChatAdapter();
const serviceAdapter = new BackendRuntimeAdapter(backendAdapter);

const runtime = new CopilotRuntime({
  actions: [],
});

app.post('/copilotkit', (req, res) => {
  const handler = copilotRuntimeNodeHttpEndpoint({
    endpoint: '/copilotkit',
    runtime,
    serviceAdapter
  });

  return handler(req, res);
});

app.listen(config.port, () => {
  console.log(`CopilotKit sidecar running on http://localhost:${config.port}`);
  console.log(`Backend URL: ${config.backendUrl}`);
  console.log(`Default model: ${config.defaultModel}`);
});
