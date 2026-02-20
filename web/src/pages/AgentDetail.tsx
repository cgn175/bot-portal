import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api, Agent } from '../api/client'

export default function AgentDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [agent, setAgent] = useState<Agent | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  const loadAgent = async () => {
    if (!id) return
    try {
      setLoading(true)
      const data = await api.getAgent(id)
      setAgent(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load agent')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadAgent()
    const interval = setInterval(loadAgent, 3000)
    return () => clearInterval(interval)
  }, [id])

  const handleAction = async (action: 'start' | 'stop' | 'restart') => {
    if (!id) return
    try {
      setError('')
      setSuccess('')
      if (action === 'start') await api.startAgent(id)
      else if (action === 'stop') await api.stopAgent(id)
      else await api.restartAgent(id)
      setSuccess(`Agent ${action}ed successfully`)
      setTimeout(loadAgent, 1000)
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to ${action} agent`)
    }
  }

  const handleDelete = async () => {
    if (!id || !confirm(`Delete agent ${id}? This cannot be undone.`)) return
    try {
      await api.deleteAgent(id)
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete agent')
    }
  }

  const copyToken = () => {
    if (agent?.bearer_token) {
      navigator.clipboard.writeText(agent.bearer_token)
      setSuccess('Bearer token copied to clipboard')
      setTimeout(() => setSuccess(''), 3000)
    }
  }

  if (loading && !agent) {
    return <div className="loading">Loading agent...</div>
  }

  if (!agent) {
    return (
      <div className="card">
        <h2>Agent not found</h2>
        <button className="btn btn-primary" onClick={() => navigate('/')}>
          Back to Dashboard
        </button>
      </div>
    )
  }

  return (
    <div>
      <button className="btn btn-secondary" onClick={() => navigate('/')} style={{ marginBottom: '1.5rem' }}>
        ← Back to Dashboard
      </button>

      {error && <div className="error-message">{error}</div>}
      {success && <div className="success-message">{success}</div>}

      <div className="card" style={{ marginBottom: '1.5rem' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'start', marginBottom: '1.5rem' }}>
          <div>
            <h2 style={{ fontSize: '2rem', fontWeight: '700', marginBottom: '0.5rem' }}>
              {agent.name}
            </h2>
            <div style={{ display: 'flex', gap: '0.5rem' }}>
              <span className={`badge ${agent.status}`}>{agent.status}</span>
              <span className="badge" style={{ background: agent.agentType === 'docker' ? 'var(--color-primary)' : 'var(--color-warning)' }}>
                {agent.agentType}
              </span>
            </div>
          </div>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            {agent.agentType === 'docker' && (
              <>
                {agent.status === 'stopped' && (
                  <button className="btn btn-primary" onClick={() => handleAction('start')}>
                    Start
                  </button>
                )}
                {agent.status === 'running' && (
                  <>
                    <button className="btn btn-secondary" onClick={() => handleAction('restart')}>
                      Restart
                    </button>
                    <button className="btn btn-secondary" onClick={() => handleAction('stop')}>
                      Stop
                    </button>
                  </>
                )}
              </>
            )}
            <button className="btn btn-danger" onClick={handleDelete}>
              Delete
            </button>
          </div>
        </div>

        {agent.description && (
          <p style={{ color: 'var(--color-text-secondary)', marginBottom: '1.5rem' }}>
            {agent.description}
          </p>
        )}

        <div style={{ display: 'grid', gap: '1rem' }}>
          <div>
            <label style={{ display: 'block', fontWeight: '600', marginBottom: '0.5rem', color: 'var(--color-text-secondary)' }}>
              Agent Type
            </label>
            <div>
              <span className="badge" style={{ background: agent.agentType === 'docker' ? 'var(--color-primary)' : 'var(--color-warning)' }}>
                {agent.agentType === 'docker' ? 'Docker Container' : 'Native Process'}
              </span>
            </div>
          </div>

          <div>
            <label style={{ display: 'block', fontWeight: '600', marginBottom: '0.5rem', color: 'var(--color-text-secondary)' }}>
              Agent ID
            </label>
            <code style={{ display: 'block', padding: '0.75rem', background: 'var(--color-bg-tertiary)', borderRadius: 'var(--radius)' }}>
              {agent.id}
            </code>
          </div>

          <div>
            <label style={{ display: 'block', fontWeight: '600', marginBottom: '0.5rem', color: 'var(--color-text-secondary)' }}>
              Endpoint
            </label>
            <code style={{ display: 'block', padding: '0.75rem', background: 'var(--color-bg-tertiary)', borderRadius: 'var(--radius)' }}>
              {agent.endpoint}
            </code>
          </div>

          <div>
            <label style={{ display: 'block', fontWeight: '600', marginBottom: '0.5rem', color: 'var(--color-text-secondary)' }}>
              Docker Image
            </label>
            <code style={{ display: 'block', padding: '0.75rem', background: 'var(--color-bg-tertiary)', borderRadius: 'var(--radius)' }}>
              {agent.image || 'N/A'}
            </code>
          </div>

          {agent.bearer_token && (
            <div>
              <label style={{ display: 'block', fontWeight: '600', marginBottom: '0.5rem', color: 'var(--color-text-secondary)' }}>
                Bearer Token
              </label>
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                <code style={{ flex: 1, padding: '0.75rem', background: 'var(--color-bg-tertiary)', borderRadius: 'var(--radius)', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {agent.bearer_token}
                </code>
                <button className="btn btn-secondary" onClick={copyToken}>
                  Copy
                </button>
              </div>
            </div>
          )}

          {agent.created_at && (
            <div>
              <label style={{ display: 'block', fontWeight: '600', marginBottom: '0.5rem', color: 'var(--color-text-secondary)' }}>
                Created
              </label>
              <div style={{ color: 'var(--color-text-muted)' }}>
                {new Date(agent.created_at).toLocaleString()}
              </div>
            </div>
          )}
        </div>
      </div>

      <div className="card">
        <h3 style={{ fontSize: '1.25rem', fontWeight: '600', marginBottom: '1rem' }}>
          Quick Actions
        </h3>
        <div style={{ display: 'grid', gap: '0.75rem' }}>
          <button className="btn btn-secondary" style={{ justifyContent: 'flex-start' }}>
            View Messages
          </button>
          <button className="btn btn-secondary" style={{ justifyContent: 'flex-start' }}>
            View Channels
          </button>
          <button className="btn btn-secondary" style={{ justifyContent: 'flex-start' }}>
            Test Connection
          </button>
        </div>
      </div>
    </div>
  )
}
