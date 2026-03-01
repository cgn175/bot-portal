import React from 'react';
import { CopilotKit } from '@copilotkit/react-core';
import '@copilotkit/react-ui/styles.css';

interface CopilotProviderProps {
  children: React.ReactNode;
}

/**
 * CopilotProvider wraps the app with CopilotKit configured for self-hosted runtime.
 *
 * Uses `/api/copilot` instead of Copilot Cloud - connects to our Go backend
 * which proxies to our configured models and auth configs.
 */
export function CopilotProvider({ children }: CopilotProviderProps) {
  return (
    <CopilotKit runtimeUrl="/api/copilot">
      {children}
    </CopilotKit>
  );
}
