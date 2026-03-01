import { useCopilotAction } from '@copilotkit/react-core';
import { api } from '../api/client';

/**
 * Registers CopilotKit actions for auth config management.
 */
export function useAuthConfigActions() {
  useCopilotAction({
    name: 'createAuthConfig',
    description: 'Create a new authentication configuration for an AI provider',
    parameters: [
      {
        name: 'id',
        type: 'string',
        description: 'Unique identifier for the auth config',
        required: true,
      },
      {
        name: 'name',
        type: 'string',
        description: 'Human-readable name for this auth config',
        required: true,
      },
      {
        name: 'provider',
        type: 'string',
        description: 'Provider ID (e.g., "openai", "anthropic", "kimi")',
        required: true,
      },
      {
        name: 'authType',
        type: 'string',
        description: 'Authentication type: "bearer_token", "basic_auth", or "github_copilot_oauth"',
        required: true,
      },
      {
        name: 'apiKey',
        type: 'string',
        description: 'API key for bearer_token auth type',
        required: false,
      },
      {
        name: 'username',
        type: 'string',
        description: 'Username for basic_auth',
        required: false,
      },
      {
        name: 'password',
        type: 'string',
        description: 'Password for basic_auth',
        required: false,
      },
      {
        name: 'endpointUrl',
        type: 'string',
        description: 'Optional endpoint URL override',
        required: false,
      },
    ],
    handler: async ({ id, name, provider, authType, apiKey, username, password, endpointUrl }) => {
      const credentials: Record<string, string> = {};
      
      if (authType === 'bearer_token' && apiKey) {
        credentials.api_key = apiKey;
      } else if (authType === 'basic_auth' && username && password) {
        credentials.username = username;
        credentials.password = password;
      }

      await api.createAuthConfig({
        id,
        name,
        provider,
        authType: authType as 'bearer_token' | 'basic_auth' | 'github_copilot_oauth',
        credentials,
        endpointUrl,
      });

      return `Auth config "${name}" created successfully with ID: ${id}. Model discovery initiated.`;
    },
  });

  useCopilotAction({
    name: 'deleteAuthConfig',
    description: 'Delete an authentication configuration. This will also delete associated models.',
    parameters: [
      {
        name: 'id',
        type: 'string',
        description: 'ID of the auth config to delete',
        required: true,
      },
    ],
    handler: async ({ id }) => {
      await api.deleteAuthConfig(id);
      return `Auth config ${id} deleted successfully`;
    },
  });
}
