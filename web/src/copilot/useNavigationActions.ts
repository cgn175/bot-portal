import { useCopilotAction } from '@copilotkit/react-core';
import { useNavigate } from 'react-router-dom';

/**
 * Registers CopilotKit actions for navigation.
 */
export function useNavigationActions() {
  const navigate = useNavigate();

  useCopilotAction({
    name: 'navigateToPage',
    description: 'Navigate to a different page in Bot Portal',
    parameters: [
      {
        name: 'page',
        type: 'string',
        description: 'Page to navigate to: "agents", "models", "auth", "chat", "messages"',
        required: true,
      },
    ],
    handler: async ({ page }) => {
      const routes: Record<string, string> = {
        agents: '/',
        models: '/models',
        auth: '/auth-configs',
        chat: '/test-chat',
        messages: '/messages',
      };

      const route = routes[page.toLowerCase()];
      if (!route) {
        return `Unknown page: ${page}. Available pages: agents, models, auth, chat, messages`;
      }

      navigate(route);
      return `Navigated to ${page} page`;
    },
  });

  useCopilotAction({
    name: 'navigateToAgent',
    description: 'Navigate to a specific agent detail page',
    parameters: [
      {
        name: 'agentId',
        type: 'string',
        description: 'ID of the agent to view',
        required: true,
      },
    ],
    handler: async ({ agentId }) => {
      navigate(`/agents/${agentId}`);
      return `Navigated to agent ${agentId} detail page`;
    },
  });

  useCopilotAction({
    name: 'navigateToChannel',
    description: 'Navigate to a specific channel messages page',
    parameters: [
      {
        name: 'channelId',
        type: 'string',
        description: 'ID of the channel to view',
        required: true,
      },
    ],
    handler: async ({ channelId }) => {
      navigate(`/channels/${channelId}`);
      return `Navigated to channel ${channelId} messages page`;
    },
  });
}
