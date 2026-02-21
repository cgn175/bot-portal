import { useState, useCallback } from 'react'
import { useAgents } from '../contexts/AgentContext'
import { api } from '../api/client'
import AgentForm from '../components/AgentForm'
import AgentGrid from '../components/AgentGrid'

export default function Dashboard() {
  const { agents, loading, error, refreshAgents } = useAgents()
  const [showForm, setShowForm] = useState(false)
  const [actionError, setActionError] = useState('')

  const handleAgentCreated = useCallback(() => {
    setShowForm(false)
    refreshAgents()
  }, [refreshAgents])

  const handleDelete = useCallback(async (id: string) => {
    if (!confirm(`Delete agent ${id}?`)) return
    try {
      await api.deleteAgent(id)
      refreshAgents()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Failed to delete agent')
    }
  }, [refreshAgents])

  const handleAction = useCallback(async (id: string, action: 'start' | 'stop' | 'restart') => {
    try {
      setActionError('')
      if (action === 'start') await api.startAgent(id)
      else if (action === 'stop') await api.stopAgent(id)
      else await api.restartAgent(id)
      setTimeout(refreshAgents, 1000)
    } catch (err) {
      setActionError(err instanceof Error ? err.message : `Failed to ${action} agent`)
    }
  }, [refreshAgents])

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
      {actionError && <div className="error-message">{actionError}</div>}

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
        <AgentGrid agents={agents} onDelete={handleDelete} onAction={handleAction} />
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
