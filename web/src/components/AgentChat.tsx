import { useState, useEffect, useRef, useCallback } from 'react'
import { api, Message, TaskLog } from '../api/client'
import Alert from './Alert'

interface AgentChatProps {
  agentId: string
  agentToken?: string
}

// A2A Task response format
interface A2ATask {
  id: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'
  messages?: Message[]
  createdAt: string
  updatedAt?: string
}

interface A2ATaskResponse {
  task: A2ATask
}

// TaskUpdate from SSE stream
interface TaskUpdate {
  task_id?: string
  status?: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'
  message?: Message
}

export default function AgentChat({ agentId, agentToken }: AgentChatProps) {
  const [messages, setMessages] = useState<Message[]>([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null)
  const scrollRef = useRef<HTMLDivElement>(null)
  const esRef = useRef<EventSource | null>(null)
  const pollIntervalRef = useRef<NodeJS.Timeout | null>(null)

  const scrollToBottom = useCallback(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [])

  useEffect(() => {
    scrollToBottom()
  }, [messages, scrollToBottom])

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (esRef.current) {
        esRef.current.close()
      }
      if (pollIntervalRef.current) {
        clearInterval(pollIntervalRef.current)
      }
    }
  }, [])

  // Get channel ID for portal <-> agent communication
  const getChannelId = useCallback(() => {
    return agentId < 'portal' ? `${agentId}::portal` : `portal::${agentId}`
  }, [agentId])

  // Parse messages from task logs
  const parseMessagesFromLogs = useCallback((logs: TaskLog[]): Message[] => {
    const history: Message[] = []
    const seenContents = new Set<string>()

    // Sort logs by created_at ascending (api returns descending by default)
    if (!logs) return []
    const sortedLogs = [...logs].reverse()

    sortedLogs.forEach((log: TaskLog) => {
      if (log.messages) {
        try {
          const parsed = typeof log.messages === 'string'
            ? JSON.parse(log.messages)
            : log.messages

          if (Array.isArray(parsed)) {
            parsed.forEach((msg: Message) => {
              // Deduplicate based on content + role
              const key = `${msg.role}:${msg.content}`
              if (!seenContents.has(key)) {
                seenContents.add(key)
                history.push(msg)
              }
            })
          }
        } catch (e) {
          console.error('Failed to parse messages from log', e)
        }
      }
    })

    return history
  }, [])

  // Load chat history from channel messages
  const loadHistory = useCallback(async () => {
    try {
      const channelId = getChannelId()
      const logs = await api.getChannelMessages(channelId)
      const history = parseMessagesFromLogs(logs)
      setMessages(history)
    } catch (err) {
      console.error('Failed to load chat history', err)
    }
  }, [getChannelId, parseMessagesFromLogs])

  // Handle task completion
  const handleTaskComplete = useCallback(async () => {
    await loadHistory()
    setLoading(false)
    setActiveTaskId(null)
    if (esRef.current) {
      esRef.current.close()
      esRef.current = null
    }
    if (pollIntervalRef.current) {
      clearInterval(pollIntervalRef.current)
      pollIntervalRef.current = null
    }
  }, [loadHistory])

  // Connect to SSE stream for task updates
  const connectTaskStream = useCallback((taskId: string) => {
    // Close any existing connection
    if (esRef.current) {
      esRef.current.close()
    }

    const es = new EventSource(`/api/tasks/${taskId}/stream`)
    esRef.current = es

    es.onmessage = (event) => {
      try {
        const update: TaskUpdate = JSON.parse(event.data)
        if (update.status === 'completed' || update.status === 'failed') {
          handleTaskComplete()
        } else if (update.message) {
          // Append streaming message
          setMessages(prev => {
            const newMsg = update.message!
            // Check if message already exists
            const exists = prev.some(m =>
              m.role === newMsg.role && m.content === newMsg.content
            )
            if (exists) return prev
            return [...prev, newMsg]
          })
        }
      } catch (e) {
        // Ignore parse errors
      }
    }

    es.onerror = () => {
      // SSE error - fall back to polling
      es.close()
      esRef.current = null
    }

    return es
  }, [handleTaskComplete])

  // Poll for task updates (fallback when SSE fails)
  const pollForTaskUpdate = useCallback(async (taskId: string) => {
    if (!agentToken) return

    try {
      const response: A2ATaskResponse = await api.getTask(taskId, agentToken)
      const task = response.task
      if (task && (task.status === 'completed' || task.status === 'failed')) {
        handleTaskComplete()
      }
    } catch (err) {
      console.error('Failed to poll for task update', err)
    }
  }, [agentToken, handleTaskComplete])

  // Load history on mount and when agent changes
  useEffect(() => {
    loadHistory()
  }, [loadHistory])

  // Set up SSE or polling when there's an active task
  useEffect(() => {
    if (activeTaskId) {
      // Try SSE first
      const es = connectTaskStream(activeTaskId)

      // Set up polling as fallback (in case SSE fails)
      pollIntervalRef.current = setInterval(() => {
        // Only poll if SSE is not connected
        if (es.readyState !== EventSource.OPEN) {
          pollForTaskUpdate(activeTaskId)
        }
      }, 3000)

      // Stop after 60 seconds (timeout)
      const timeout = setTimeout(() => {
        handleTaskComplete()
        setError('Request timed out. The agent may be busy or not responding.')
      }, 60000)

      return () => {
        clearTimeout(timeout)
        if (pollIntervalRef.current) {
          clearInterval(pollIntervalRef.current)
        }
        if (esRef.current) {
          esRef.current.close()
        }
      }
    }
  }, [activeTaskId, connectTaskStream, pollForTaskUpdate, handleTaskComplete])

  const handleSend = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!input.trim() || loading) return

    const userMsg: Message = { role: 'user', content: input }
    setMessages(prev => [...prev, userMsg])
    setInput('')
    setLoading(true)
    setError('')

    try {
      const { taskId } = await api.sendAgentChatMessage(agentId, [...messages, userMsg])
      setActiveTaskId(taskId)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to send message')
      setLoading(false)
    }
  }

  return (
    <div className="card" style={{ display: 'flex', flexDirection: 'column', height: '500px' }}>
      <div className="card-header">
        <h3 className="card-title">Chat with Agent</h3>
      </div>

      <div
        ref={scrollRef}
        style={{
          flex: 1,
          overflowY: 'auto',
          padding: '1rem',
          display: 'flex',
          flexDirection: 'column',
          gap: '1rem'
        }}
      >
        {messages.length === 0 && (
          <div style={{ textAlign: 'center', color: 'var(--color-text-muted)', marginTop: '2rem' }}>
            No messages yet. Start a conversation!
          </div>
        )}

        {messages.map((msg, i) => (
          <div
            key={i}
            style={{
              alignSelf: msg.role === 'user' ? 'flex-end' : 'flex-start',
              maxWidth: '80%',
              padding: '0.75rem 1rem',
              borderRadius: 'var(--radius-md)',
              background: msg.role === 'user' ? 'var(--color-primary)' : 'var(--color-bg)',
              color: msg.role === 'user' ? 'white' : 'var(--color-text)',
              border: msg.role === 'user' ? 'none' : '1px solid var(--color-border)',
              boxShadow: 'var(--shadow-sm)',
              whiteSpace: 'pre-wrap'
            }}
          >
            {msg.content}
          </div>
        ))}

        {loading && (
          <div style={{ alignSelf: 'flex-start', padding: '0.75rem 1rem', color: 'var(--color-text-muted)' }}>
            Agent is thinking...
          </div>
        )}
      </div>

      {error && (
        <div style={{ padding: '0 1rem' }}>
          <Alert type="error" onClose={() => setError('')}>{error}</Alert>
        </div>
      )}

      <form
        onSubmit={handleSend}
        style={{
          padding: '1rem',
          borderTop: '1px solid var(--color-border)',
          display: 'flex',
          gap: '0.5rem'
        }}
      >
        <input
          type="text"
          value={input}
          onChange={e => setInput(e.target.value)}
          placeholder="Type a message..."
          style={{
            flex: 1,
            padding: '0.75rem',
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--color-border)',
            background: 'var(--color-bg)',
            color: 'var(--color-text)'
          }}
          disabled={loading}
        />
        <button
          type="submit"
          className="btn btn-primary"
          disabled={loading || !input.trim()}
        >
          Send
        </button>
      </form>
    </div>
  )
}
