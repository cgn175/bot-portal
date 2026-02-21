import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { api, Agent } from '../api/client'
import AgentForm from '../components/AgentForm'

export default function Dashboard() {
  const [agents, setAgents] = useState<Agent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)

  const loadAgents = async () => {
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
    loadAgents()

    // Use SSE for real-time updates
    const eventSource = new EventSource('/api/agents-stream')
    
    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        // Don't update if form is open to prevent losing user input
        if (!showForm) {
          setAgents(data || [])
          setLoading(false)
        }
      } catch (err) {
        console.error('Failed to parse SSE data:', err)
      }
    }

    eventSource.onerror = () => {
      eventSource.close()
      // Fallback to polling if SSE fails
      const interval = setInterval(() => {
        if (!showForm) loadAgents()
      }, 10000)
      return () => clearInterval(interval)
    }

    return () => eventSource.close()
  }, [showForm])

  const handleAgentCreated = () => {
    setShowForm(false)
    loadAgents()
  }

  const handleDelete = async (id: string) => {
    if (!confirm(`Delete agent ${id}?`)) return
    try {
      await api.deleteAgent(id)
      loadAgents()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete agent')
    }
  }

  const handleAction = async (id: string, action: 'start' | 'stop' | 'restart') => {
    try {
      setError('')
      if (action === 'start') await api.startAgent(id)
      else if (action === 'stop') await api.stopAgent(id)
      else await api.restartAgent(id)
      setTimeout(loadAgents, 1000)
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to ${action} agent`)
    }
  }

  if (loading && agents.length === 0) {
    return <div className="loading">Loading agents...</div>
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '2rem' }}>
        <h2 style={{ fontSize: '2rem', fontWeight: '700' }}>Agents</h2>
        <button className="btn btn-primary" onClick={() => setShowForm(true)}>
          + Register Agent
        </button>
      </div>

      {error && <div className="error-message">{error}</div>}

      {agents.length === 0 ? (
        <div className="card" style={{ textAlign: 'center', padding: '3rem' }}>
          <p style={{ color: 'var(--color-text-muted)', marginBottom: '1rem' }}>
            No agents registered yet
          </p>
          <button className="btn btn-primary" onClick={() => setShowForm(true)}>
            Register Your First Agent
          </button>
        </div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))', gap: '1.5rem' }}>
          {agents.map(agent => (
            <div key={agent.id} className="card">
              <div style={{ marginBottom: '1rem' }}>
                <h3 style={{ fontSize: '1.25rem', fontWeight: '600', marginBottom: '0.5rem' }}>
                  {agent.name}
                </h3>
                <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
                  <span className={`badge ${agent.status}`}>{agent.status}</span>
                  <span className="badge" style={{ background: agent.agentType === 'docker' ? 'var(--color-primary)' : 'var(--color-warning)' }}>
                    {agent.agentType}
                  </span>
                </div>
              </div>
              
              {agent.description && (
                <p style={{ color: 'var(--color-text-secondary)', fontSize: '0.875rem', marginBottom: '1rem' }}>
                  {agent.description}
                </p>
              )}
              
              <div style={{ fontSize: '0.875rem', color: 'var(--color-text-muted)', marginBottom: '1rem' }}>
                <div style={{ marginBottom: '0.25rem' }}>
                  <strong>ID:</strong> <code>{agent.id}</code>
                </div>
                <div style={{ marginBottom: '0.25rem' }}>
                  <strong>Endpoint:</strong> <code>{agent.endpoint}</code>
                </div>
                <div>
                  <strong>Image:</strong> <code>{agent.image}</code>
                </div>
              </div>

              <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
                <Link to={`/agents/${agent.id}`} className="btn btn-secondary" style={{ textDecoration: 'none', flex: 1 }}>
                  Details
                </Link>
                {agent.agentType === 'docker' && (
                  <>
                    {agent.status === 'stopped' && (
                      <button className="btn btn-primary" onClick={() => handleAction(agent.id, 'start')}>
                        Start
                      </button>
                    )}
                    {agent.status === 'running' && (
                      <>
                        <button className="btn btn-secondary" onClick={() => handleAction(agent.id, 'restart')}>
                          Restart
                        </button>
                        <button className="btn btn-secondary" onClick={() => handleAction(agent.id, 'stop')}>
                          Stop
                        </button>
                      </>
                    )}
                  </>
                )}
                <button className="btn btn-danger" onClick={() => handleDelete(agent.id)}>
                  Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {showForm && (
        <AgentForm
          onSuccess={handleAgentCreated}
          onCancel={() => setShowForm(false)}
        />
      )}
    </div>
  )
}
