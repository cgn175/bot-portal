import React, { useEffect, useState } from 'react';
import { CopilotKit } from '@copilotkit/react-core';
import '@copilotkit/react-ui/styles.css';
import { api } from '../api/client';

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
  const [defaultModel, setDefaultModel] = useState<string | undefined>();

  useEffect(() => {
    // Fetch default model from backend on mount
    api.getCopilotKitSettings()
      .then(settings => {
        if (settings.defaultModel) {
          setDefaultModel(settings.defaultModel);
        }
      })
      .catch(err => console.error('Failed to load CopilotKit settings:', err));
  }, []);

  return (
    <CopilotKit
      runtimeUrl={runtimeUrl}
      showDevConsole={import.meta.env.DEV}
    >
      {children}
    </CopilotKit>
  );
}
