// Load environment variables from .env (must be first)
import "dotenv/config";

import express from "express";
import cors from "cors";
import {
  copilotRuntimeNodeHttpEndpoint,
  CopilotRuntime,
} from "@copilotkit/runtime";
import { BuiltInAgent } from "@copilotkitnext/agent";
import { createOpenAI } from "@ai-sdk/openai";
import { config } from "./config.js";
import { BackendChatAdapter } from "./backend-adapter.js";
import { BackendRuntimeAdapter } from "./runtime-adapter.js";

const app = express();

app.use(express.json());

app.use(
  cors({
    origin: config.corsOrigin,
    credentials: true,
  }),
);

app.get("/health", (req, res) => {
  res.json({ status: "ok", service: "bot-portal-copilot-sidecar" });
});

// Create an OpenAI-compatible model that routes to the Go backend
// instead of directly to OpenAI. The backend handles real API key resolution.
const backendOpenAI = createOpenAI({
  baseURL: `${config.backendUrl}/api/copilotkit`,
  apiKey: "backend-managed",
});
const backendModel = backendOpenAI.chat(config.defaultModel || "gpt-4o-mini");

// Create backend adapter and wire to runtime
const backendAdapter = new BackendChatAdapter();
const serviceAdapter = new BackendRuntimeAdapter(backendAdapter);

const runtime = new CopilotRuntime({
  actions: [],
  agents: {
    default: new BuiltInAgent({ model: backendModel }),
  },
});

app.post("/copilotkit", (req, res) => {
  const handler = copilotRuntimeNodeHttpEndpoint({
    endpoint: "/copilotkit",
    runtime,
    serviceAdapter,
  });

  return handler(req, res);
});

app.listen(config.port, () => {
  console.log(`CopilotKit sidecar running on http://localhost:${config.port}`);
  for (let cfg of Object.keys(config)) {
    console.log(cfg, config[cfg]);
  }
});
