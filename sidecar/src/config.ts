export const config = {
  // Sidecar server port
  port: process.env.COPILOT_PORT || 3001,

  // Bot Portal Go backend base URL
  backendUrl: process.env.BACKEND_URL || 'http://localhost:8080',

  // CORS allowed origin (React dev server)
  corsOrigin: process.env.CORS_ORIGIN || 'http://localhost:5173',
};
