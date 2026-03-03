import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

const env = loadEnv("", process.cwd());
export default defineConfig({
  plugins: [react()],
  server: {
    port: 3100,
    proxy: {
      "/copilotkit": env.VITE_COPILOTKIT_SERVER,
      "/api": env.VITE_BACKEND_URL,
      "/tasks": env.VITE_BACKEND_URL,
      "/.well-known": env.VITE_BACKEND_URL,
    },
  },
});
/*
Why is import.meta.env undefined here?

- import.meta.env is a Vite feature that is populated for modules that Vite serves/bundles (client code).
  It is NOT automatically populated when Node loads this file (e.g. when vite reads vite.config.ts).
  If you inspect import.meta.env at Node runtime (or in a plain ts-node execution), it may be undefined.

- In vite.config.ts you should not rely on import.meta.env at top-level. Instead, use Vite's loadEnv
  (or process.env for Node-level variables) to read environment variables while Vite starts.

Common fixes / recommended approach:

1) Use loadEnv inside the exported config function:

import { loadEnv } from "vite";

export default defineConfig(({ mode }) => {
  // load .env, .env.[mode] files and merge into an object
  const env = loadEnv(mode, process.cwd());

  return {
    plugins: [react()],
    server: {
      port: 3100,
      proxy: {
        "/copilotkit": env.VITE_COPILOTKIT_SERVER,
        "/api": env.VITE_BACKEND_URL,
        "/tasks": env.VITE_BACKEND_URL,
        "/.well-known": env.VITE_BACKEND_URL,
      },
    },
  };
});

2) If you need values in client-side code, ensure variable names are prefixed with VITE_.
   Vite only exposes env variables that start with VITE_ to client bundles (import.meta.env).
   For example: VITE_BACKEND_URL in your .env file becomes import.meta.env.VITE_BACKEND_URL in the browser.

3) When debugging in Node (e.g. using ts-node, editors, or tests), prefer process.env or loadEnv
   rather than import.meta.env since the latter is not a standard Node runtime property.

Summary:
- import.meta.env undefined in vite.config.ts because Node/Vite config execution context doesn't provide the client-side import.meta.env.
- Use loadEnv(mode, cwd) inside defineConfig to read environment variables when building the Vite config.
*/
