import { useCopilotAction } from '@copilotkit/react-core';
import { api } from '../api/client';
import { useAgents } from '../contexts/AgentContext';

/**
 * Registers CopilotKit actions for agent management.
 * These actions allow the AI assistant to manage agents via function calling.
 */
export function useAgentActions() {
  const { refreshAgents } = useAgents();

  useCopilotAction({
    name: 'createAgent',
    description: 'Create a new AI agent in Bot Portal',
    parameters: [
      {
        name: 'id',
        type: 'string',
        description: 'Unique identifier for the agent (e.g., "my-agent")',
        required: true,
      },
      {
        name: 'name',
        type: 'string',
        description: 'Human-readable name for the agent',
        required: true,
      },
      {
        name: 'image',
        type: 'string',
        description: 'Docker image for the agent (e.g., "agent:latest")',
        required: true,
      },
      {
        name: 'agentType',
        type: 'string',
        description: 'Type of agent: "docker" or "native"',
        required: true,
      },
      {
        name: 'endpoint',
        type: 'string',
        description: 'Optional endpoint URL for native agents',
        required: false,
      },
      {
        name: 'description',
        type: 'string',
        description: 'Optional description of what this agent does',
        required: false,
      },
      {
        name: 'modelId',
        type: 'string',
        description: 'Optional model ID to use for this agent',
        required: false,
      },
      {
        name: 'authConfigId',
        type: 'string',
        description: 'Optional auth config ID for this agent',
        required: false,
      },
    ],
    handler: async ({ id, name, image, agentType, endpoint, description, modelId, authConfigId }) => {
      await api.createAgent({
        id,
        name,
        image,
        agentType: agentType as 'docker' | 'native',
        endpoint,
        description,
        modelId,
        authConfigId,
      });
      await refreshAgents();
      return `Agent "${name}" created successfully with ID: ${id}`;
    },
  });

  useCopilotAction({
    name: 'deleteAgent',
    description: 'Delete an agent from Bot Portal. Use this carefully as it cannot be undone.',
    parameters: [
      {
        name: 'id',
        type: 'string',
        description: 'ID of the agent to delete',
        required: true,
      },
    ],
    handler: async ({ id }) => {
      await api.deleteAgent(id);
      await refreshAgents();
      return `Agent ${id} deleted successfully`;
    },
  });

  useCopilotAction({
    name: 'startAgent',
    description: 'Start a stopped agent. This will create and run the agent container.',
    parameters: [
      {
        name: 'id',
        type: 'string',
        description: 'ID of the agent to start',
        required: true,
      },
    ],
    handler: async ({ id }) => {
      await api.startAgent(id);
      await refreshAgents();
      return `Agent ${id} started successfully`;
    },
  });

  useCopilotAction({
    name: 'stopAgent',
    description: 'Stop a running agent. This will stop and remove the agent container.',
    parameters: [
      {
        name: 'id',
        type: 'string',
        description: 'ID of the agent to stop',
        required: true,
      },
    ],
    handler: async ({ id }) => {
      await api.stopAgent(id);
      await refreshAgents();
      return `Agent ${id} stopped successfully`;
    },
  });

  useCopilotAction({
    name: 'restartAgent',
    description: 'Restart a running agent. This will stop and then start the agent.',
    parameters: [
      {
        name: 'id',
        type: 'string',
        description: 'ID of the agent to restart',
        required: true,
      },
    ],
    handler: async ({ id }) => {
      await api.restartAgent(id);
      await refreshAgents();
      return `Agent ${id} restarted successfully`;
    },
  });

  useCopilotAction({
    name: 'updateAgent',
    description: 'Update an existing agent configuration',
    parameters: [
      {
        name: 'id',
        type: 'string',
        description: 'ID of the agent to update',
        required: true,
      },
      {
        name: 'name',
        type: 'string',
        description: 'New name for the agent',
        required: false,
      },
      {
        name: 'description',
        type: 'string',
        description: 'New description',
        required: false,
      },
      {
        name: 'modelId',
        type: 'string',
        description: 'New model ID',
        required: false,
      },
      {
        name: 'authConfigId',
        type: 'string',
        description: 'New auth config ID',
        required: false,
      },
    ],
    handler: async ({ id, name, description, modelId, authConfigId }) => {
      const updates: Record<string, unknown> = {};
      if (name !== undefined) updates.name = name;
      if (description !== undefined) updates.description = description;
      if (modelId !== undefined) updates.modelId = modelId;
      if (authConfigId !== undefined) updates.authConfigId = authConfigId;

      await api.updateAgent(id, updates);
      await refreshAgents();
      return `Agent ${id} updated successfully`;
    },
  });
}
