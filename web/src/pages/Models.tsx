import { useState, useEffect, useCallback } from 'react'
import { api, Model, AuthConfig } from '../api/client'
import Alert from '../components/Alert'
import EmptyState from '../components/EmptyState'
import { SkeletonCard } from '../components/LoadingState'
import ModelForm from '../components/ModelForm'

export default function Models() {
  const [models, setModels] = useState<Model[]>([])
  const [authConfigs, setAuthConfigs] = useState<AuthConfig[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [editingModel, setEditingModel] = useState<Model | undefined>()
  const [syncing, setSyncing] = useState(false)

  const fetchData = useCallback(async () => {
    try {
      setLoading(true)
      const [modelsData, configsData] = await Promise.all([
        api.listModels(),
        api.listAuthConfigs()
      ])
      setModels(modelsData)
      setAuthConfigs(configsData)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch models')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  const handleModelSaved = useCallback(() => {
    setShowForm(false)
    setEditingModel(undefined)
    fetchData()
  }, [fetchData])

  const handleEdit = useCallback((model: Model) => {
    setEditingModel(model)
    setShowForm(true)
  }, [])

  const handleDelete = useCallback(async (id: string) => {
    if (!confirm(`Delete model ${id}?`)) return
    try {
      await api.deleteModel(id)
      fetchData()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Failed to delete model')
    }
  }, [fetchData])

  const handleDeleteAll = useCallback(async () => {
    if (!confirm('Are you sure you want to delete ALL models? This action cannot be undone.')) return
    try {
      await api.deleteAllModels()
      fetchData()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Failed to delete all models')
    }
  }, [fetchData])

  const handleCancel = useCallback(() => {
    setShowForm(false)
    setEditingModel(undefined)
  }, [])

  const handleSync = useCallback(async () => {
    setSyncing(true)
    setActionError('')
    try {
      // Trigger re-discovery by re-saving each auth config (update with no changes)
      // This triggers the backend's discoverAndSaveModels
      for (const config of authConfigs) {
        await api.updateAuthConfig(config.id, {})
      }
      // Wait a moment for async discovery to complete, then refresh
      await new Promise(resolve => setTimeout(resolve, 2000))
      await fetchData()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Failed to sync models')
    } finally {
      setSyncing(false)
    }
  }, [authConfigs, fetchData])

  if (loading && (!models || models?.length === 0)) {
    return (
      <div>
        <div className="page-header">
          <h2>Models</h2>
        </div>
        <SkeletonCard count={3} />
      </div>
    )
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h2>Models</h2>
          <p style={{ color: 'var(--color-text-muted)', marginTop: '0.5rem' }}>
            Models are automatically discovered from your auth providers
          </p>
        </div>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          {authConfigs?.length > 0 && (
            <button
              className="btn btn-secondary"
              onClick={handleSync}
              disabled={syncing}
            >
              {syncing ? 'Syncing...' : '↻ Sync from Providers'}
            </button>
          )}
          {models?.length > 0 && (
            <button
              className="btn btn-danger"
              onClick={handleDeleteAll}
            >
              Delete All
            </button>
          )}
          <button className="btn btn-primary" onClick={() => setShowForm(true)}>
            <span>+</span>
            <span>Add Model</span>
          </button>
        </div>
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
 
      {(!models || (models?.length ?? 0) === 0) ? (
        <EmptyState
          icon="🧠"
          title="No models configured yet"
          description={
            (authConfigs?.length ?? 0) > 0
              ? 'Models will be auto-discovered when you add an auth provider. Click "Sync from Providers" to refresh.'
              : 'Set up an auth provider first — models will be auto-discovered, or add one manually.'
          }
          action={
            (authConfigs?.length ?? 0) > 0 ? (
              <button className="btn btn-primary" onClick={handleSync} disabled={syncing}>
                {syncing ? 'Syncing...' : 'Sync from Providers'}
              </button>
            ) : (
              <button className="btn btn-primary" onClick={() => setShowForm(true)}>
                Add Model Manually
              </button>
            )
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
                <th>Model Identifier</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {models?.map((model) => {
                const config = authConfigs?.find(c => c.id === model.authConfigId)
                const provider = config 
                  ? (config.authType === 'github_copilot_oauth' ? 'copilot' : config.provider) 
                  : 'unknown'
                
                return (
                  <tr key={model.id}>
                    <td>
                      <code>{model.id}</code>
                    </td>
                    <td>{model.name}</td>
                    <td>
                      <ProviderBadge provider={provider} />
                    </td>
                    <td>
                      <code>{model.modelIdentifier}</code>
                    </td>
                    <td>
                      <div className="action-buttons">
                        <button
                          className="btn btn-sm btn-secondary"
                          onClick={() => handleEdit(model)}
                        >
                          Edit
                        </button>
                        <button
                          className="btn btn-sm btn-danger"
                          onClick={() => handleDelete(model.id)}
                        >
                          Delete
                        </button>
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      {showForm && (
        <ModelForm
          model={editingModel}
          onSuccess={handleModelSaved}
          onCancel={handleCancel}
        />
      )}
    </div>
  )
}

function ProviderBadge({ provider }: { provider: string }) {
  const providerColors: Record<string, string> = {
    openai: '#10a37f',
    anthropic: '#d4a574',
    gemini: '#4285f4',
    copilot: '#6e7681',
    custom: '#8b5cf6',
    ollama: '#ff6b35',
    openrouter: '#ef4444'
  }

  const color = providerColors[provider.toLowerCase()] || 'var(--color-text-muted)'

  return (
    <span
      className="badge"
      style={{
        background: `${color}20`,
        color: color,
        textTransform: 'capitalize'
      }}
    >
      {provider}
    </span>
  )
}
