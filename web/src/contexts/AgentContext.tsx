import { createContext, useContext, useEffect, useRef, ReactNode, useState } from 'react'
import { Agent, api } from '../api/client'

interface AgentContextType {
  agents: Agent[]
  loading: boolean
  error: string
  refreshAgents: () => Promise<void>
}

const AgentContext = createContext<AgentContextType | undefined>(undefined)

export function AgentProvider({ children }: { children: ReactNode }) {
  const [agents, setAgents] = useState<Agent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const eventSourceRef = useRef<EventSource | null>(null)

  const refreshAgents = async () => {
    try {
      setLoading(true)
      setError('')
      const data = await api.listAgents()
      setAgents(data || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load agents')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    refreshAgents()

    eventSourceRef.current = new EventSource('/api/agents-stream')
    
    eventSourceRef.current.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        setAgents(prev => {
          if (JSON.stringify(prev) !== JSON.stringify(data)) {
            return data || []
          }
          return prev
        })
        setLoading(false)
      } catch (err) {
        console.error('Failed to parse SSE data:', err)
      }
    }

    eventSourceRef.current.onerror = () => {
      eventSourceRef.current?.close()
    }

    return () => {
      eventSourceRef.current?.close()
    }
  }, [])

  return (
    <AgentContext.Provider value={{ agents, loading, error, refreshAgents }}>
      {children}
    </AgentContext.Provider>
  )
}

export function useAgents() {
  const context = useContext(AgentContext)
  if (!context) {
    throw new Error('useAgents must be used within AgentProvider')
  }
  return context
}
