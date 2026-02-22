export interface Agent {
  id: string
  name: string
  description?: string
  agentType: 'docker' | 'native'
  status: 'running' | 'stopped' | 'error' | 'pending'
  endpoint: string
  image: string
  bearer_token?: string
  created_at?: string
  updated_at?: string
}

export interface Channel {
  id: string
  members: string[]
  created_at?: string
}

export interface Message {
  role: 'user' | 'assistant' | 'system'
  content: string
}

export interface TaskLog {
  id: string
  channel_id: string
  sender_id: string
  recipient_id?: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'
  direction: 'inbound' | 'outbound'
  messages?: Message[]
  created_at: string
  updated_at?: string
}

export interface CreateAgentRequest {
  id: string
  name: string
  image: string
  agentType: 'docker' | 'native'
  endpoint: string
  description?: string
}

export interface CreateTaskRequest {
  channel_id: string
  sender_id: string
  message: Message
}

// Model types
export interface Model {
  id: string
  name: string
  provider: string
  modelIdentifier: string
  endpointUrl?: string
  defaultParams?: string
  createdAt?: string
  updatedAt?: string
}

export interface CreateModelRequest {
  id: string
  name: string
  provider: string
  modelName: string
  baseUrl?: string
  apiKeyConfig?: Record<string, unknown>
}

// Auth Config types
export interface AuthConfig {
  id: string
  name: string
  provider: string
  authType: 'bearer_token' | 'basic_auth' | 'github_copilot_oauth'
  credentials: string
  endpointUrl?: string
  createdAt?: string
  updatedAt?: string
}

export interface CreateAuthConfigRequest {
  id: string
  name: string
  provider: string
  authType: 'bearer_token' | 'basic_auth' | 'github_copilot_oauth'
  credentials: Record<string, string>
  endpointUrl?: string
}

// Copilot OAuth types
export interface DeviceCodeResponse {
  device_code: string
  user_code: string
  verification_uri: string
  expires_in: number
  interval: number
}

// TokenResult matches the backend response structure
export interface TokenResult {
  success: boolean
  message?: string
  configId?: string
}

const API_BASE = '/api'

class ApiClient {
  async listAgents(): Promise<Agent[]> {
    const res = await fetch(`${API_BASE}/agents`)
    if (!res.ok) throw new Error('Failed to fetch agents')
    return res.json()
  }

  async getAgent(id: string): Promise<Agent> {
    const res = await fetch(`${API_BASE}/agents/${id}`)
    if (!res.ok) throw new Error('Failed to fetch agent')
    return res.json()
  }

  async createAgent(data: CreateAgentRequest): Promise<Agent> {
    const res = await fetch(`${API_BASE}/agents`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    })
    if (!res.ok) {
      const error = await res.text()
      throw new Error(error || 'Failed to create agent')
    }
    return res.json()
  }

  async updateAgent(id: string, data: Partial<CreateAgentRequest>): Promise<Agent> {
    const res = await fetch(`${API_BASE}/agents/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    })
    if (!res.ok) throw new Error('Failed to update agent')
    return res.json()
  }

  async deleteAgent(id: string): Promise<void> {
    const res = await fetch(`${API_BASE}/agents/${id}`, { method: 'DELETE' })
    if (!res.ok) throw new Error('Failed to delete agent')
  }

  async startAgent(id: string): Promise<void> {
    const res = await fetch(`${API_BASE}/agents/${id}?action=start`, { method: 'POST' })
    if (!res.ok) throw new Error('Failed to start agent')
  }

  async stopAgent(id: string): Promise<void> {
    const res = await fetch(`${API_BASE}/agents/${id}?action=stop`, { method: 'POST' })
    if (!res.ok) throw new Error('Failed to stop agent')
  }

  async restartAgent(id: string): Promise<void> {
    const res = await fetch(`${API_BASE}/agents/${id}?action=restart`, { method: 'POST' })
    if (!res.ok) throw new Error('Failed to restart agent')
  }

  async listChannels(): Promise<Channel[]> {
    const res = await fetch(`${API_BASE}/channels`)
    if (!res.ok) throw new Error('Failed to fetch channels')
    return res.json()
  }

  async getChannel(id: string): Promise<Channel> {
    const res = await fetch(`${API_BASE}/channels/${id}`)
    if (!res.ok) throw new Error('Failed to fetch channel')
    return res.json()
  }

  async getChannelMessages(id: string): Promise<TaskLog[]> {
    const res = await fetch(`${API_BASE}/channels/${id}/messages`)
    if (!res.ok) throw new Error('Failed to fetch messages')
    return res.json()
  }

  async createTask(data: CreateTaskRequest, token: string): Promise<TaskLog> {
    const res = await fetch('/tasks', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`
      },
      body: JSON.stringify(data)
    })
    if (!res.ok) throw new Error('Failed to create task')
    return res.json()
  }

  async getTask(id: string, token: string): Promise<TaskLog> {
    const res = await fetch(`/tasks/${id}`, {
      headers: { 'Authorization': `Bearer ${token}` }
    })
    if (!res.ok) throw new Error('Failed to fetch task')
    return res.json()
  }

  streamMessages(channelId?: string): EventSource {
    const url = channelId
      ? `${API_BASE}/messages/stream?channel_id=${channelId}`
      : `${API_BASE}/messages/stream`
    return new EventSource(url)
  }

  // ============================================================================
  // Model Management
  // ============================================================================

  async listModels(): Promise<Model[]> {
    const res = await fetch(`${API_BASE}/models`)
    if (!res.ok) throw new Error('Failed to fetch models')
    return res.json()
  }

  async getModel(id: string): Promise<Model> {
    const res = await fetch(`${API_BASE}/models/${id}`)
    if (!res.ok) throw new Error('Failed to fetch model')
    return res.json()
  }

  async createModel(data: CreateModelRequest): Promise<Model> {
    const res = await fetch(`${API_BASE}/models`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    })
    if (!res.ok) {
      const error = await res.text()
      throw new Error(error || 'Failed to create model')
    }
    return res.json()
  }

  async updateModel(id: string, data: Partial<CreateModelRequest>): Promise<Model> {
    const res = await fetch(`${API_BASE}/models/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    })
    if (!res.ok) throw new Error('Failed to update model')
    return res.json()
  }

  async deleteModel(id: string): Promise<void> {
    const res = await fetch(`${API_BASE}/models/${id}`, { method: 'DELETE' })
    if (!res.ok) throw new Error('Failed to delete model')
  }

  // ============================================================================
  // Auth Config Management
  // ============================================================================

  async listAuthConfigs(): Promise<AuthConfig[]> {
    const res = await fetch(`${API_BASE}/auth-configs`)
    if (!res.ok) throw new Error('Failed to fetch auth configs')
    return res.json()
  }

  async getAuthConfig(id: string): Promise<AuthConfig> {
    const res = await fetch(`${API_BASE}/auth-configs/${id}`)
    if (!res.ok) throw new Error('Failed to fetch auth config')
    return res.json()
  }

  async createAuthConfig(data: CreateAuthConfigRequest): Promise<AuthConfig> {
    const res = await fetch(`${API_BASE}/auth-configs`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    })
    if (!res.ok) {
      const error = await res.text()
      throw new Error(error || 'Failed to create auth config')
    }
    return res.json()
  }

  async updateAuthConfig(id: string, data: Partial<CreateAuthConfigRequest>): Promise<AuthConfig> {
    const res = await fetch(`${API_BASE}/auth-configs/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    })
    if (!res.ok) throw new Error('Failed to update auth config')
    return res.json()
  }

  async deleteAuthConfig(id: string): Promise<void> {
    const res = await fetch(`${API_BASE}/auth-configs/${id}`, { method: 'DELETE' })
    if (!res.ok) throw new Error('Failed to delete auth config')
  }

  // ============================================================================
  // GitHub Copilot OAuth Device Flow
  // ============================================================================

  async initiateCopilotDeviceFlow(): Promise<DeviceCodeResponse> {
    const res = await fetch(`${API_BASE}/auth/copilot/device-code`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    })
    if (!res.ok) throw new Error('Failed to initiate device flow')
    return res.json()
  }

  async pollCopilotToken(deviceCode: string): Promise<TokenResult | null> {
    const res = await fetch(`${API_BASE}/auth/copilot/token`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ device_code: deviceCode })
    })
    if (res.status === 202) {
      // Still waiting for user authorization
      return null
    }
    if (!res.ok) throw new Error('Failed to get token')
    return res.json()
  }
}

export const api = new ApiClient()
