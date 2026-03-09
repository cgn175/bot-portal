import { useState, useEffect, useCallback } from 'react'
import { api, AuthConfig } from '../api/client'
import Alert from '../components/Alert'
import EmptyState from '../components/EmptyState'
import { SkeletonCard } from '../components/LoadingState'
import AuthConfigForm from './AuthConfigForm'

export default function AuthConfigs() {
  const [configs, setConfigs] = useState<AuthConfig[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [editingConfig, setEditingConfig] = useState<AuthConfig | undefined>()

  const fetchConfigs = useCallback(async () => {
    try {
      setLoading(true)
      const data = await api.listAuthConfigs()
      setConfigs(data)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch auth configs')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchConfigs()
  }, [fetchConfigs])

  const handleConfigSaved = useCallback(() => {
    setShowForm(false)
    setEditingConfig(undefined)
    fetchConfigs()
  }, [fetchConfigs])

  const handleEdit = useCallback((config: AuthConfig) => {
    setEditingConfig(config)
    setShowForm(true)
  }, [])

  const handleDelete = useCallback(async (id: string) => {
    if (!confirm(`Delete auth config ${id}?`)) return
    try {
      await api.deleteAuthConfig(id)
      fetchConfigs()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Failed to delete auth config')
    }
  }, [fetchConfigs])

  const handleCancel = useCallback(() => {
    setShowForm(false)
    setEditingConfig(undefined)
  }, [])

  if (loading && (!configs || configs?.length === 0)) {
    return (
      <div>
        <div className="page-header">
          <h2>Auth Configs</h2>
        </div>
        <SkeletonCard count={3} />
      </div>
    )
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h2>Auth Configs</h2>
          <p style={{ color: 'var(--color-text-muted)', marginTop: '0.5rem' }}>
            Manage authentication credentials for AI providers
          </p>
        </div>
        <button className="btn btn-primary" onClick={() => setShowForm(true)}>
          <span>+</span>
          <span>Add Auth Config</span>
        </button>
      </div>

      {error && (
        <Alert type="error" onClose={() => setError('')}>
          {error}
        </Alert>
      )}

      {actionError && (
        <Alert type="error" onClose={() => setActionError('')}>
          {actionError}
        </Alert>
      )}

      {!configs || configs.length === 0 ? (
        <EmptyState
          icon="🔐"
          title="No auth configs yet"
          description="Add authentication credentials to enable your agents to connect to AI providers."
          action={
            <button className="btn btn-primary" onClick={() => setShowForm(true)}>
              Add Your First Auth Config
            </button>
          }
        />
      ) : (
        <div className="table-container">
          <table className="data-table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Name</th>
                <th>Provider</th>
                <th>Auth Type</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {configs?.map((config) => (
                <tr key={config.id}>
                  <td>
                    <code>{config.id}</code>
                  </td>
                  <td>{config.name}</td>
                  <td>
                    <span style={{ textTransform: 'capitalize' }}>{config.provider}</span>
                  </td>
                  <td>
                    <AuthTypeBadge authType={config.authType} />
                  </td>
                  <td>
                    <div className="action-buttons">
                      <button
                        className="btn btn-sm btn-secondary"
                        onClick={() => handleEdit(config)}
                      >
                        Edit
                      </button>
                      <button
                        className="btn btn-sm btn-danger"
                        onClick={() => handleDelete(config.id)}
                      >
                        Delete
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showForm && (
        <AuthConfigForm
          config={editingConfig}
          onSuccess={handleConfigSaved}
          onCancel={handleCancel}
        />
      )}
    </div>
  )
}

function AuthTypeBadge({ authType }: { authType: string }) {
  const authTypeLabels: Record<string, string> = {
    bearer_token: 'Custom',
    basic_auth: 'Custom',
    github_copilot_oauth: 'GitHub Copilot'
  }

  const authTypeColors: Record<string, { bg: string; color: string }> = {
    bearer_token: { bg: 'var(--color-primary-subtle)', color: 'var(--color-primary)' },
    basic_auth: { bg: 'var(--color-primary-subtle)', color: 'var(--color-primary)' },
    github_copilot_oauth: { bg: 'var(--color-success-subtle)', color: 'var(--color-success)' }
  }

  const style = authTypeColors[authType] || { bg: 'var(--color-bg-tertiary)', color: 'var(--color-text-muted)' }

  return (
    <span
      className="badge"
      style={{
        background: style.bg,
        color: style.color
      }}
    >
      {authTypeLabels[authType] || authType}
    </span>
  )
}
