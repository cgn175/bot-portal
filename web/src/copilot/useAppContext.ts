import { useCopilotReadable } from '@copilotkit/react-core';
import { useAgents } from '../contexts/AgentContext';
import { useState, useEffect } from 'react';
import { api, Model, AuthConfig, Provider } from '../api/client';

/**
 * Exposes agents state to CopilotKit.
 * Updates whenever agents list changes.
 */
export function useAgentsContext() {
  const { agents } = useAgents();

  useCopilotReadable({
    description: 'List of AI agents in Bot Portal with their status, type, and configuration',
    value: agents.map(agent => ({
      id: agent.id,
      name: agent.name,
      description: agent.description,
      status: agent.status,
      type: agent.agentType,
      modelId: agent.modelId,
      authConfigId: agent.authConfigId,
      endpoint: agent.endpoint,
      image: agent.image,
    })),
  });
}

/**
 * Exposes models state to CopilotKit.
 */
export function useModelsContext() {
  const [models, setModels] = useState<Model[]>([]);

  useEffect(() => {
    api.listModels().then(setModels).catch(console.error);
  }, []);

  useCopilotReadable({
    description: 'List of AI models configured in Bot Portal with provider and model identifier',
    value: models.map(model => ({
      id: model.id,
      name: model.name,
      provider: model.provider,
      modelIdentifier: model.modelIdentifier,
      endpointUrl: model.endpointUrl,
    })),
  });
}

/**
 * Exposes auth configs state to CopilotKit.
 */
export function useAuthConfigsContext() {
  const [authConfigs, setAuthConfigs] = useState<AuthConfig[]>([]);

  useEffect(() => {
    api.listAuthConfigs().then(setAuthConfigs).catch(console.error);
  }, []);

  useCopilotReadable({
    description: 'List of authentication configurations with provider and auth type',
    value: authConfigs.map(config => ({
      id: config.id,
      name: config.name,
      provider: config.provider,
      authType: config.authType,
    })),
  });
}

/**
 * Exposes available providers to CopilotKit.
 */
export function useProvidersContext() {
  const [providers, setProviders] = useState<Provider[]>([]);

  useEffect(() => {
    api.listProviders().then(setProviders).catch(console.error);
  }, []);

  useCopilotReadable({
    description: 'List of available AI providers that can be configured',
    value: providers.map(provider => ({
      id: provider.id,
      name: provider.name,
      authType: provider.authType,
      defaultUrl: provider.defaultUrl,
      description: provider.description,
    })),
  });
}
