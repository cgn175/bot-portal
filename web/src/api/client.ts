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
}

export const api = new ApiClient()
