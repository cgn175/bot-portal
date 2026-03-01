import express from 'express';
import cors from 'cors';
import { copilotRuntimeNodeHttpEndpoint } from '@copilotkit/runtime';
import { CopilotRuntime, EmptyAdapter } from '@copilotkit/runtime';
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

app.use('/copilot', (req, res) => {
  const runtime = new CopilotRuntime({
    actions: [],
  });

  // TODO: Wire up the adapter to CopilotRuntime's service adapter interface
  // This will be completed in Task 4

  const handler = copilotRuntimeNodeHttpEndpoint({
    endpoint: '/copilot',
    runtime,
  });

  return handler(req, res);
});

app.listen(config.port, () => {
  console.log(`CopilotKit sidecar running on http://localhost:${config.port}`);
  console.log(`Backend URL: ${config.backendUrl}`);
});
