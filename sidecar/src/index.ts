import express from 'express';
import cors from 'cors';
import { copilotRuntimeNodeHttpEndpoint, CopilotRuntime, EmptyAdapter } from '@copilotkit/runtime';
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
// For now, use EmptyAdapter as a stub - we'll implement the backend adapter in next task
const runtime = new CopilotRuntime({
  actions: [],
  agents: [
    {
      name: 'bot-portal-agent',
      description: 'Bot Portal AI Agent',
    },
  ],
});

app.use('/copilot', copilotRuntimeNodeHttpEndpoint({
  endpoint: '/copilot',
  runtime,
  serviceAdapter: new EmptyAdapter(),
}));

app.listen(config.port, () => {
  console.log(`CopilotKit sidecar running on http://localhost:${config.port}`);
  console.log(`Backend URL: ${config.backendUrl}`);
  console.log(`CORS origin: ${config.corsOrigin}`);
});
