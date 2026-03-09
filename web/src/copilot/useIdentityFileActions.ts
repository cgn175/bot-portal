import { useCopilotAction } from '@copilotkit/react-core';
import { api } from '../api/client';

/**
 * Registers CopilotKit actions for identity file management.
 * These actions allow the AI assistant to read and update agent identity files.
 */
export function useIdentityFileActions() {
  useCopilotAction({
    name: 'listIdentityFiles',
    description: 'List all identity files for a specific agent. Returns filenames and whether they have content.',
    parameters: [
      {
        name: 'agentId',
        type: 'string',
        description: 'ID of the agent whose identity files to list',
        required: true,
      },
    ],
    handler: async ({ agentId }) => {
      const response = await api.getAgentIdentityFiles(agentId);
      const files = response.files || [];
      if (files.length === 0) {
        return `No identity files found for agent ${agentId}.`;
      }
      const summary = files.map(f => `- ${f.filename} (${f.charCount > 0 ? `${f.charCount} chars` : 'empty'})`).join('\n');
      return `Identity files for agent ${agentId}:\n${summary}`;
    },
  });

  useCopilotAction({
    name: 'getIdentityFile',
    description: 'Read the content of a specific identity file for an agent. Valid filenames: IDENTITY.md, SOUL.md, AGENTS.md, USER.md, TOOLS.md, config.toml',
    parameters: [
      {
        name: 'agentId',
        type: 'string',
        description: 'ID of the agent',
        required: true,
      },
      {
        name: 'filename',
        type: 'string',
        description: 'Name of the identity file (e.g., "IDENTITY.md", "SOUL.md")',
        required: true,
      },
    ],
    handler: async ({ agentId, filename }) => {
      const response = await api.getIdentityFile(agentId, filename);
      if (!response.content) {
        return `File "${filename}" for agent ${agentId} is empty.`;
      }
      return `Content of ${filename} for agent ${agentId}:\n\n${response.content}`;
    },
  });

  useCopilotAction({
    name: 'updateIdentityFile',
    description: 'Update the content of an identity file for an agent. Valid filenames: IDENTITY.md, SOUL.md, AGENTS.md, USER.md, TOOLS.md. config.toml is read-only. Content must be under 16KB.',
    parameters: [
      {
        name: 'agentId',
        type: 'string',
        description: 'ID of the agent',
        required: true,
      },
      {
        name: 'filename',
        type: 'string',
        description: 'Name of the identity file to update (e.g., "IDENTITY.md", "SOUL.md")',
        required: true,
      },
      {
        name: 'content',
        type: 'string',
        description: 'New markdown content for the file',
        required: true,
      },
    ],
    handler: async ({ agentId, filename, content }) => {
      if (filename === 'config.toml') {
        return 'Error: config.toml is read-only and cannot be updated.';
      }
      if (content.length > 16 * 1024) {
        return 'Error: Content exceeds the 16KB size limit. Please reduce the content.';
      }
      await api.updateIdentityFile(agentId, filename, content);
      return `Identity file "${filename}" for agent ${agentId} updated successfully. Changes are now active.`;
    },
  });
}
