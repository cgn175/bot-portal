import { useAgentsContext, useModelsContext, useAuthConfigsContext, useProvidersContext } from './useAppContext';
import { useAgentActions } from './useAgentActions';
import { useModelActions } from './useModelActions';
import { useAuthConfigActions } from './useAuthConfigActions';
import { useNavigationActions } from './useNavigationActions';
import { useIdentityFileActions } from './useIdentityFileActions';

/**
 * Renderless component that exposes all app state and actions to CopilotKit.
 * Place this inside CopilotProvider to make context available to the assistant.
 */
export function CopilotActions() {
  // Context hooks - expose app state
  useAgentsContext();
  useModelsContext();
  useAuthConfigsContext();
  useProvidersContext();

  // Action hooks - expose operations
  useAgentActions();
  useModelActions();
  useAuthConfigActions();
  useNavigationActions();
  useIdentityFileActions();

  return null;
}
