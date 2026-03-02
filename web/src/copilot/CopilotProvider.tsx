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
  const runtimeUrl = import.meta.env.VITE_COPILOT_RUNTIME_URL || 'http://localhost:3001/copilotkit';

  // Get default model from localStorage (set in ModelForm when "Set as CopilotKit default" is checked)
  const defaultModel = localStorage.getItem('copilotkit_default_model') || undefined;

  return (
    <CopilotKit
      runtimeUrl={runtimeUrl}
      showDevConsole={import.meta.env.DEV}
      properties={{
        ...(defaultModel && { model: defaultModel })
      }}
    >
      {children}
    </CopilotKit>
  );
}
