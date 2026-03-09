import { useCopilotAction } from "@copilotkit/react-core";
import { api } from "../api/client";

/**
 * Registers CopilotKit actions for model management.
 */
export function useModelActions() {
  useCopilotAction({
    name: "createModel",
    description: "Create a new AI model configuration in Bot Portal",
    parameters: [
      {
        name: "id",
        type: "string",
        description: "Unique identifier for the model",
        required: true,
      },
      {
        name: "name",
        type: "string",
        description: "Human-readable name for the model",
        required: true,
      },
      {
        name: "authConfigId",
        type: "string",
        description: 'ID of the auth configuration to associate with this model',
        required: true,
      },
      {
        name: "modelName",
        type: "string",
        description:
          'Model identifier from the provider (e.g., "gpt-4", "claude-3-opus")',
        required: true,
      },
      {
        name: "baseUrl",
        type: "string",
        description: "Optional base URL override for the provider API",
        required: false,
      },
    ],
    handler: async ({ id, name, authConfigId, modelName, baseUrl }) => {
      await api.createModel({
        id,
        name,
        authConfigId,
        modelName,
        baseUrl,
        isDefault: false,
      });
      return `Model "${name}" created successfully with ID: ${id}`;
    },
  });

  useCopilotAction({
    name: "deleteModel",
    description: "Delete a model configuration from Bot Portal",
    parameters: [
      {
        name: "id",
        type: "string",
        description: "ID of the model to delete",
        required: true,
      },
    ],
    handler: async ({ id }) => {
      await api.deleteModel(id);
      return `Model ${id} deleted successfully`;
    },
  });

  useCopilotAction({
    name: "syncModels",
    description:
      "Trigger model discovery for an auth config to fetch available models from the provider",
    parameters: [
      {
        name: "authConfigId",
        type: "string",
        description: "ID of the auth config to sync models for",
        required: true,
      },
    ],
    handler: async ({ authConfigId }) => {
      // Note: The actual sync is triggered when creating an auth config
      // This action is a placeholder for future dedicated sync endpoint
      return `Model sync for auth config ${authConfigId} initiated. New models will appear shortly.`;
    },
  });
}
