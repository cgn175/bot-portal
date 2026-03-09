import { useState, useCallback } from 'react'
import { useAgents } from '../contexts/AgentContext'
import { api } from '../api/client'
import AgentForm from '../components/AgentForm'
import AgentGrid from '../components/AgentGrid'
import Alert from '../components/Alert'
import { SkeletonCard } from '../components/LoadingState'
import EmptyState from '../components/EmptyState'

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

  if (loading && (agents?.length ?? 0) === 0) {
    return (
      <div>
        <div className="page-header">
          <h2>Agents</h2>
        </div>
        <SkeletonCard count={3} />
      </div>
    )
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h2>Agents</h2>
          <p style={{ color: 'var(--color-text-muted)', marginTop: '0.5rem' }}>
            Manage and monitor your AI agents
          </p>
        </div>
        <button className="btn btn-primary" onClick={() => setShowForm(true)}>
          <span>+</span>
          <span>Register Agent</span>
        </button>
      </div>

      {error && (
        <Alert type="error" onClose={() => window.location.reload()}>
          {error}
        </Alert>
      )}

      {actionError && (
        <Alert type="error" onClose={() => setActionError('')}>
          {actionError}
        </Alert>
      )}

      {(agents?.length ?? 0) === 0 ? (
        <EmptyState
          icon="🤖"
          title="No agents registered yet"
          description="Get started by registering your first agent to manage your AI workflows."
          action={
            <button className="btn btn-primary" onClick={() => setShowForm(true)}>
              Register Your First Agent
            </button>
          }
        />
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
