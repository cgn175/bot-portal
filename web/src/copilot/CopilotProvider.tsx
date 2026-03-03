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
  const runtimeUrl = 'http://localhost:3001/copilotkit';
  return (
    <CopilotKit
      runtimeUrl={runtimeUrl}
      showDevConsole={import.meta.env.DEV}
    >
      {children}
    </CopilotKit>
  );
}
