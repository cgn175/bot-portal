function parsePort(val: unknown, fallback: number) {
  const n = Number(val);
  return Number.isFinite(n) && n > 0 ? n : fallback;
}

function parseCorsOrigin(val: unknown, fallback: string[] | string) {
  if (!val) return fallback;
  if (Array.isArray(val)) return val;
  if (typeof val === 'string') {
    // Try JSON first (e.g. '["http://localhost:3100","http://localhost:5173"]')
    try {
      const parsed = JSON.parse(val);
      if (Array.isArray(parsed)) return parsed;
    } catch (e) {
      // ignore
    }
    // Comma-separated
    return val.split(',').map(s => s.trim()).filter(Boolean);
  }
  return fallback;
}

export const config = {
  // Sidecar server port
  port: parsePort(process.env.COPILOT_PORT, 3001),

  // Bot Portal Go backend base URL
  backendUrl: process.env.BACKEND_URL || 'http://localhost:8080',

  // CORS allowed origin (React dev server)
  // Support both default vite port (5173) and custom port (3100)
  corsOrigin: parseCorsOrigin(process.env.CORS_ORIGIN, ['http://localhost:3100', 'http://localhost:5173']),

  // Default model to use if not specified by CopilotKit
  // This should be a model ID from your Bot Portal database
  defaultModel: process.env.DEFAULT_MODEL || 'gpt-5-mini',
};
