import { useState, useEffect, useCallback } from 'react'
import { api, Model } from '../api/client'
import Alert from '../components/Alert'
import EmptyState from '../components/EmptyState'
import { SkeletonCard } from '../components/LoadingState'
import ModelForm from './ModelForm'

export default function Models() {
  const [models, setModels] = useState<Model[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [editingModel, setEditingModel] = useState<Model | undefined>()

  const fetchModels = useCallback(async () => {
    try {
      setLoading(true)
      const data = await api.listModels()
      setModels(data)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch models')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchModels()
  }, [fetchModels])

  const handleModelSaved = useCallback(() => {
    setShowForm(false)
    setEditingModel(undefined)
    fetchModels()
  }, [fetchModels])

  const handleEdit = useCallback((model: Model) => {
    setEditingModel(model)
    setShowForm(true)
  }, [])

  const handleDelete = useCallback(async (id: string) => {
    if (!confirm(`Delete model ${id}?`)) return
    try {
      await api.deleteModel(id)
      fetchModels()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Failed to delete model')
    }
  }, [fetchModels])

  const handleCancel = useCallback(() => {
    setShowForm(false)
    setEditingModel(undefined)
  }, [])

  if (loading && models.length === 0) {
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
            Manage AI model configurations for your agents
          </p>
        </div>
        <button className="btn btn-primary" onClick={() => setShowForm(true)}>
          <span>+</span>
          <span>Add Model</span>
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

      {models.length === 0 ? (
        <EmptyState
          icon="🧠"
          title="No models configured yet"
          description="Add your first model configuration to enable AI capabilities for your agents."
          action={
            <button className="btn btn-primary" onClick={() => setShowForm(true)}>
              Add Your First Model
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
                <th>Model Identifier</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {models.map((model) => (
                <tr key={model.id}>
                  <td>
                    <code>{model.id}</code>
                  </td>
                  <td>{model.name}</td>
                  <td>
                    <ProviderBadge provider={model.provider} />
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
              ))}
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
