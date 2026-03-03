import { useState, useCallback, useEffect } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useAgents } from '../contexts/AgentContext'
import { api, Model, AuthConfig } from '../api/client'
import AgentForm from '../components/AgentForm'
import AgentChat from '../components/AgentChat'
import Alert from '../components/Alert'
import StatusBadge from '../components/StatusBadge'
import { LoadingState } from '../components/LoadingState'
import EmptyState from '../components/EmptyState'

export default function AgentDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { agents, loading, refreshAgents } = useAgents()
  const agent = agents.find(a => a.id === id)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [pinging, setPinging] = useState(false)
  const [showEditForm, setShowEditForm] = useState(false)
  const [models, setModels] = useState<Model[]>([])
  const [authConfigs, setAuthConfigs] = useState<AuthConfig[]>([])

  useEffect(() => {
    const loadMetadata = async () => {
      try {
        const [modelsData, authConfigsData] = await Promise.all([
          api.listModels(),
          api.listAuthConfigs()
        ])
        setModels(modelsData)
        setAuthConfigs(authConfigsData)
      } catch (err) {
        console.error('Failed to load metadata in AgentDetail', err)
      }
    }
    loadMetadata()
  }, [])

  const handleAction = useCallback(async (action: 'start' | 'stop' | 'restart' | 'recreate') => {
    if (!id) return
    try {
      setError('')
      setSuccess('')
      switch (action) {
        case 'start':
          await api.startAgent(id)
          break
        case 'stop':
          await api.stopAgent(id)
          break
        case 'restart':
          await api.restartAgent(id)
          break
        case 'recreate':
          await api.recreateAgent(id)
          break
      }
      setSuccess(`Agent ${action}ed successfully`)
      setTimeout(refreshAgents, 1000)
      setTimeout(() => setSuccess(''), 5000)
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to ${action} agent`)
    }
  }, [id, refreshAgents])

  const handleDelete = useCallback(async () => {
    if (!id || !confirm(`Delete agent ${id}? This cannot be undone.`)) return
    try {
      await api.deleteAgent(id)
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete agent')
    }
  }, [id, navigate])

  const handlePing = useCallback(async () => {
    if (!id) return
    try {
      setError('')
      setSuccess('')
      setPinging(true)
      const result = await api.pingAgent(id)
      if (result.online) {
        setSuccess('Agent is online and responding')
      } else {
        setError(`Agent is offline: ${result.error || 'Connection failed'}`)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to test connection')
    } finally {
      setPinging(false)
    }
  }, [id])

  const copyToken = useCallback(() => {
    if (agent?.bearer_token) {
      navigator.clipboard.writeText(agent.bearer_token)
      setSuccess('Bearer token copied to clipboard')
      setTimeout(() => setSuccess(''), 3000)
    }
  }, [agent?.bearer_token])

  const handleEditSuccess = useCallback(() => {
    setShowEditForm(false)
    setSuccess('Agent updated successfully')
    setTimeout(() => setSuccess(''), 3000)
    refreshAgents()
  }, [refreshAgents])

  if (loading && !agent) {
    return <LoadingState message="Loading agent details..." />
  }

  if (!agent) {
    return (
      <EmptyState
        icon="🔍"
        title="Agent not found"
        description="The agent you're looking for doesn't exist or has been removed."
        action={
          <Link to="/" className="btn btn-primary" style={{ textDecoration: 'none' }}>
            Back to Dashboard
          </Link>
        }
      />
    )
  }

  const isDocker = agent.agentType === 'docker'
  const selectedModel = models.find(m => m.id === agent.modelId)
  const selectedAuth = authConfigs.find(c => c.id === agent.authConfigId)

  return (
    <div>
      <div style={{ marginBottom: '1.5rem' }}>
        <Link
          to="/"
          className="btn btn-ghost"
          style={{ textDecoration: 'none', paddingLeft: 0 }}
        >
          ← Back to Dashboard
        </Link>
      </div>

      {error && (
        <Alert type="error" onClose={() => setError('')}>
          {error}
        </Alert>
      )}

      {success && (
        <Alert type="success" onClose={() => setSuccess('')}>
          {success}
        </Alert>
      )}

      <div className="card" style={{ marginBottom: '1.5rem' }}>
        <div className="card-header" style={{ marginBottom: '1.5rem' }}>
          <div>
            <h2 style={{ fontSize: 'var(--font-size-3xl)', fontWeight: 'var(--font-weight-bold)', marginBottom: '0.75rem' }}>
              {agent.name}
            </h2>
            <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
              <StatusBadge status={agent.status} />
              <StatusBadge status={agent.agentType} />
            </div>
          </div>
          <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
            <button className="btn btn-secondary" onClick={() => setShowEditForm(true)}>
              Edit
            </button>

            {isDocker && agent.status === 'stopped' && (
              <button className="btn btn-primary" onClick={() => handleAction('start')}>
                Start
              </button>
            )}

            {isDocker && agent.status === 'running' && (
              <>
                <button className="btn btn-secondary" onClick={() => handleAction('restart')}>
                  Restart
                </button>
                <button className="btn btn-secondary" onClick={() => handleAction('stop')}>
                  Stop
                </button>
              </>
            )}

            {isDocker && agent.status === 'running' && (
                <>
                  <button className="btn btn-secondary" onClick={() => handleAction('recreate')}>
                    Recreate Container
                  </button>
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

        <div style={{ display: 'grid', gap: '1.25rem' }}>
          <InfoRow label="Agent ID" value={<code>{agent.id}</code>} />
          <InfoRow label="Agent Type" value={<StatusBadge status={agent.agentType} />} />
          <InfoRow
            label="Endpoint"
            value={<code>{agent.endpoint || (isDocker ? `http://${agent.id}:8080` : 'N/A')}</code>}
          />
          <InfoRow label="Docker Image" value={<code>{agent.image || 'N/A'}</code>} />
          
          <InfoRow 
            label="Auth Provider" 
            value={
              selectedAuth ? (
                <span>
                  {selectedAuth.name}{' '}
                  <span style={{ color: 'var(--color-text-muted)', fontSize: 'var(--font-size-sm)' }}>
                    ({selectedAuth.authType === 'github_copilot_oauth' ? 'GitHub Copilot' : 'Custom'})
                  </span>
                </span>
              ) : (
                <span style={{ color: 'var(--color-text-muted)' }}>{agent.authConfigId || 'None'}</span>
              )
            } 
          />

          <InfoRow 
            label="Model" 
            value={
              selectedModel ? (
                <span>
                  {selectedModel.name}{' '}
                  <span style={{ color: 'var(--color-text-muted)', fontSize: 'var(--font-size-sm)' }}>
                    ({selectedModel.modelIdentifier})
                  </span>
                </span>
              ) : (
                <span style={{ color: 'var(--color-text-muted)' }}>{agent.modelId || 'None'}</span>
              )
            } 
          />

          {agent.bearer_token && (
            <div>
              <label style={labelStyle}>
                Bearer Token
              </label>
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                <code
                  style={{
                    flex: 1,
                    padding: '0.75rem 1rem',
                    background: 'var(--color-bg)',
                    borderRadius: 'var(--radius-md)',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                    border: '1px solid var(--color-border)'
                  }}
                >
                  {agent.bearer_token}
                </code>
                <button className="btn btn-secondary" onClick={copyToken}>
                  Copy
                </button>
              </div>
            </div>
          )}

          {agent.created_at && (
            <InfoRow
              label="Created"
              value={
                <span style={{ color: 'var(--color-text-muted)' }}>
                  {new Date(agent.created_at).toLocaleString()}
                </span>
              }
            />
          )}

          {agent.updated_at && (
            <InfoRow
              label="Last Updated"
              value={
                <span style={{ color: 'var(--color-text-muted)' }}>
                  {new Date(agent.updated_at).toLocaleString()}
                </span>
              }
            />
          )}
        </div>
      </div>

      {/* Chat with Agent - Only show for running agents */}
      {agent.status === 'running' && (
        <div style={{ marginBottom: '1.5rem' }}>
          <AgentChat agentId={agent.id} agentToken={agent.bearer_token} />
        </div>
      )}

      <div className="card" style={{ marginBottom: '1.5rem' }}>
        <h3 style={{ fontSize: 'var(--font-size-xl)', fontWeight: 'var(--font-weight-semibold)', marginBottom: '1rem' }}>
          Quick Actions
        </h3>
        <div style={{ display: 'grid', gap: '0.75rem' }}>
          <Link
            to="/messages"
            className="btn btn-secondary"
            style={{ textDecoration: 'none', justifyContent: 'flex-start' }}
          >
            📨 View Messages
          </Link>
          <button
            className="btn btn-secondary"
            style={{ justifyContent: 'flex-start' }}
            onClick={handlePing}
            disabled={pinging}
          >
            {pinging ? '⏳ Testing...' : '🔗 Test Connection'}
          </button>
          <Link
            to={`/agents/${id}/identity`}
            className="btn btn-secondary"
            style={{ textDecoration: 'none', justifyContent: 'flex-start' }}
          >
            📝 Edit Identity Files
          </Link>
        </div>
      </div>

      {showEditForm && (
        <AgentForm
          agent={agent}
          onSuccess={handleEditSuccess}
          onCancel={() => setShowEditForm(false)}
        />
      )}
    </div>
  )
}

const labelStyle = {
  display: 'block',
  fontWeight: 'var(--font-weight-medium)',
  marginBottom: '0.5rem',
  color: 'var(--color-text-secondary)',
  fontSize: 'var(--font-size-sm)'
}

interface InfoRowProps {
  label: string
  value: React.ReactNode
}

function InfoRow({ label, value }: InfoRowProps) {
  return (
    <div>
      <label style={labelStyle}>{label}</label>
      <div>{value}</div>
    </div>
  )
}
