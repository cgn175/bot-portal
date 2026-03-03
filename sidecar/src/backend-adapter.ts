import { config } from "./config.js";

export interface ChatMessage {
  role: "system" | "user" | "assistant";
  content: string;
}

export interface ChatRequest {
  model?: string;
  messages: ChatMessage[];
  stream: boolean;
  tools?: any[];
  tool_choice?: any;
}

/**
 * BackendChatAdapter forwards chat requests to the Go backend's
 * /api/copilotkit/chat/completions endpoint and streams back responses.
 */
export class BackendChatAdapter {
  private backendUrl: string;

  constructor(backendUrl: string = config.backendUrl) {
    this.backendUrl = backendUrl;
  }

  /**
   * Send chat completion request to Go backend.
   * Returns async generator that yields SSE chunks.
   */
  async *streamChatCompletion(request: ChatRequest): AsyncGenerator<string> {
    const url = `${this.backendUrl}/chat/completions`;

    console.log("makeing request");
    const response = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Inject-System-Prompt": "true",
      },
      body: JSON.stringify({
        ...request,
        stream: true, // Always stream
      }),
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`Backend error: ${response.status} - ${errorText}`);
    }

    if (!response.body) {
      throw new Error("No response body from backend");
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";

    try {
      while (true) {
        const { done, value } = await reader.read();

        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() || "";

        for (const line of lines) {
          if (line.trim() === "") continue;
          if (line.startsWith("data: ")) {
            const data = line.slice(6);
            if (data === "[DONE]") {
              return;
            }
            yield data;
          }
        }
      }
    } finally {
      reader.releaseLock();
    }
  }

  /**
   * Get available models from backend.
   */
  async getAvailableModels(): Promise<any[]> {
    const url = `${this.backendUrl}/info`;
    const response = await fetch(url);

    if (!response.ok) {
      throw new Error(`Failed to fetch models: ${response.status}`);
    }

    const data = await response.json();
    return data.models || [];
  }
}
