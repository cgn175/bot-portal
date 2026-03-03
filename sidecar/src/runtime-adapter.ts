import {
  CopilotServiceAdapter,
  CopilotRuntimeChatCompletionRequest,
  CopilotRuntimeChatCompletionResponse
} from '@copilotkit/runtime';
import { randomUUID } from 'crypto';
import { createOpenAI } from '@ai-sdk/openai';
import { LanguageModel } from 'ai';
import { BackendChatAdapter } from './backend-adapter.js';
import { config } from './config.js';

/**
 * Adapter that bridges CopilotKit's runtime to our Go backend.
 * Implements the CopilotServiceAdapter interface.
 */
export class BackendRuntimeAdapter implements CopilotServiceAdapter {
  provider = 'openai';
  model?: string;
  private languageModel: LanguageModel;

  constructor(private backendAdapter: BackendChatAdapter, defaultModel?: string) {
    this.model = defaultModel || config.defaultModel;

    // Create an OpenAI-compatible model that routes to our Go backend
    // instead of directly to OpenAI. The backend handles real API key resolution.
    const backendOpenAI = createOpenAI({
      baseURL: `${config.backendUrl}/api/copilotkit`,
      apiKey: 'backend-managed',
    });
    // Use .chat() to force the Chat Completions API (/chat/completions)
    // instead of the default Responses API (/responses)
    this.languageModel = backendOpenAI.chat(this.model || 'gpt-4o-mini');
  }

  getLanguageModel(): LanguageModel {
    return this.languageModel;
  }

  async process(request: CopilotRuntimeChatCompletionRequest): Promise<CopilotRuntimeChatCompletionResponse> {
    const { messages, model, threadId: threadIdFromRequest, eventSource } = request;

    const threadId = threadIdFromRequest ?? randomUUID();

    // Transform CopilotKit messages to backend format
    const backendMessages = messages
      .filter((msg) => msg.isTextMessage())
      .map((msg) => {
        if (msg.isTextMessage()) {
          return {
            role: msg.role as 'system' | 'user' | 'assistant',
            content: msg.content,
          };
        }
        // Fallback (should never reach here due to filter)
        return {
          role: 'user' as const,
          content: '',
        };
      });

    // Stream from backend using the event source
    await eventSource.stream(async (eventStream$) => {
      let currentMessageId = randomUUID();
      let messageStarted = false;

      try {
        // Start text message
        eventStream$.sendTextMessageStart({
          messageId: currentMessageId
        });
        messageStarted = true;

        // Stream from backend
        for await (const chunk of this.backendAdapter.streamChatCompletion({
          model: model || this.model,
          messages: backendMessages,
          stream: true,
        })) {
          try {
            const parsed = JSON.parse(chunk);

            // Transform OpenAI streaming format to CopilotKit events
            if (parsed.choices && parsed.choices[0]?.delta?.content) {
              const content = parsed.choices[0].delta.content;
              eventStream$.sendTextMessageContent({
                messageId: currentMessageId,
                content,
              });
            }

            // Handle tool calls if present (future enhancement)
            if (parsed.choices && parsed.choices[0]?.delta?.tool_calls) {
              // End current text message if started
              if (messageStarted) {
                eventStream$.sendTextMessageEnd({ messageId: currentMessageId });
                messageStarted = false;
              }

              for (const toolCall of parsed.choices[0].delta.tool_calls) {
                const toolCallId = toolCall.id || randomUUID();

                eventStream$.sendActionExecutionStart({
                  actionExecutionId: toolCallId,
                  actionName: toolCall.function?.name || 'unknown',
                });

                if (toolCall.function?.arguments) {
                  eventStream$.sendActionExecutionArgs({
                    actionExecutionId: toolCallId,
                    args: toolCall.function.arguments,
                  });
                }

                eventStream$.sendActionExecutionEnd({
                  actionExecutionId: toolCallId,
                });
              }
            }
          } catch (e) {
            console.error('Failed to parse SSE chunk:', chunk, e);
          }
        }

        // End text message if still active
        if (messageStarted) {
          eventStream$.sendTextMessageEnd({ messageId: currentMessageId });
        }

      } catch (error: any) {
        console.error('Error in backend streaming:', error);

        // Send error event
        eventStream$.next({
          type: 'RunError' as any,
          message: error.message || 'Unknown error occurred',
          code: error.code || 'BACKEND_ERROR',
        });
      }
    });

    return {
      threadId,
    };
  }
}
