import { useState, useEffect } from 'react'
import { api, CreateModelRequest, Model, AuthConfig, CopilotModel } from '../api/client'
import Modal from '../components/Modal'
import Alert from '../components/Alert'

interface ModelFormProps {
  model?: Model
  onSuccess: () => void
  onCancel: () => void
}

const DEFAULT_PARAMS_EXAMPLE = `{
  "temperature": 0.7,
  "max_tokens": 4096
}`

export default function ModelForm({ model, onSuccess, onCancel }: ModelFormProps) {
  const [formData, setFormData] = useState<CreateModelRequest>({
    id: '',
    name: '',
    provider: '',
    modelName: '',
    baseUrl: '',
    apiKeyConfig: {}
  })
  const [defaultParams, setDefaultParams] = useState(DEFAULT_PARAMS_EXAMPLE)
  const [setCopilotDefault, setSetCopilotDefault] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  // Auth config integration
  const [authConfigs, setAuthConfigs] = useState<AuthConfig[]>([])
  const [selectedAuthConfig, setSelectedAuthConfig] = useState('')
  const [availableModels, setAvailableModels] = useState<CopilotModel[]>([])
  const [browsing, setBrowsing] = useState(false)

  useEffect(() => {
    api.listAuthConfigs().then(setAuthConfigs).catch(() => {})
  }, [])

  useEffect(() => {
    if (model) {
      setFormData({
        id: model.id,
        name: model.name,
        provider: model.provider,
        modelName: model.modelIdentifier,
        baseUrl: model.endpointUrl || '',
        apiKeyConfig: {}
      })
      if (model.defaultParams) {
        try {
          const parsed = JSON.parse(model.defaultParams)
          setDefaultParams(JSON.stringify(parsed, null, 2))
        } catch {
          setDefaultParams(model.defaultParams)
        }
      }
    }
  }, [model])

  const handleAuthConfigChange = (configId: string) => {
    setSelectedAuthConfig(configId)
    setAvailableModels([])
    if (!configId) return

    const config = authConfigs.find(c => c.id === configId)
    if (!config) return

    const provider = config.authType === 'github_copilot_oauth' ? 'copilot' : config.provider
    setFormData(prev => ({
      ...prev,
      provider,
      baseUrl: config.endpointUrl || ''
    }))
  }

  const handleBrowseModels = async () => {
    if (!selectedAuthConfig) return
    setBrowsing(true)
    try {
      const discovered = await api.discoverModels(selectedAuthConfig)
      setAvailableModels(discovered)
    } catch {
      setError('Failed to discover models from this provider')
    } finally {
      setBrowsing(false)
    }
  }

  const handleSelectModel = (m: CopilotModel) => {
    setFormData(prev => ({
      ...prev,
      id: prev.id || m.id,
      name: prev.name || m.name || m.id,
      modelName: m.id
    }))
    setAvailableModels([])
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      // Parse and validate default params JSON
      let parsedParams: Record<string, unknown> | undefined
      if (defaultParams.trim()) {
        try {
          parsedParams = JSON.parse(defaultParams)
        } catch {
          setError('Invalid JSON in Default Params')
          setLoading(false)
          return
        }
      }

      const submitData: CreateModelRequest = {
        ...formData,
        apiKeyConfig: parsedParams
      }

      if (model) {
        await api.updateModel(model.id, submitData)
      } else {
        await api.createModel(submitData)
      }

      // If "Set as CopilotKit default" was checked, save to localStorage
      if (setCopilotDefault) {
        localStorage.setItem('copilotkit_default_model', formData.id)
      }

      onSuccess()
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to ${model ? 'update' : 'create'} model`)
    } finally {
      setLoading(false)
    }
  }

  const footer = (
    <>
      <button
        type="button"
        className="btn btn-secondary"
        onClick={onCancel}
        disabled={loading}
      >
        Cancel
      </button>
      <button
        type="submit"
        className="btn btn-primary"
        disabled={loading}
        form="model-form"
      >
        {loading ? (model ? 'Updating...' : 'Creating...') : (model ? 'Update Model' : 'Create Model')}
      </button>
    </>
  )

  return (
    <Modal
      isOpen={true}
      onClose={onCancel}
      title={model ? 'Edit Model' : 'Add New Model'}
      footer={footer}
      size="lg"
    >
      {error && (
        <Alert type="error" onClose={() => setError('')}>
          {error}
        </Alert>
      )}

      <form id="model-form" onSubmit={handleSubmit}>
        {!model && authConfigs.length > 0 && (
          <div className="form-group">
            <label htmlFor="authConfig">Auth Provider</label>
            <select
              id="authConfig"
              value={selectedAuthConfig}
              onChange={e => handleAuthConfigChange(e.target.value)}
            >
              <option value="">— Select to auto-fill —</option>
              {authConfigs.map(c => (
                <option key={c.id} value={c.id}>{c.name}</option>
              ))}
            </select>
            <small>Select a provider to auto-fill endpoint and discover models</small>
          </div>
        )}

        <div className="form-group">
          <label htmlFor="modelName">Model Identifier *</label>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <input
              id="modelName"
              type="text"
              required
              value={formData.modelName}
              onChange={e => setFormData({ ...formData, modelName: e.target.value })}
              placeholder="gpt-4o"
              style={{ flex: 1 }}
            />
            {selectedAuthConfig && (
              <button
                type="button"
                className="btn btn-secondary"
                onClick={handleBrowseModels}
                disabled={browsing}
                style={{ whiteSpace: 'nowrap' }}
              >
                {browsing ? 'Loading...' : 'Browse'}
              </button>
            )}
          </div>
          {availableModels.length > 0 && (
            <div style={{
              marginTop: '0.5rem',
              maxHeight: '200px',
              overflowY: 'auto',
              border: '1px solid var(--color-border)',
              borderRadius: 'var(--radius-md)',
              background: 'var(--color-bg)'
            }}>
              {availableModels.map(m => (
                <button
                  key={m.id}
                  type="button"
                  onClick={() => handleSelectModel(m)}
                  style={{
                    display: 'block',
                    width: '100%',
                    padding: '0.375rem 0.75rem',
                    background: formData.modelName === m.id ? 'var(--color-primary-subtle)' : 'none',
                    border: 'none',
                    borderBottom: '1px solid var(--color-border-subtle)',
                    textAlign: 'left',
                    cursor: 'pointer',
                    fontSize: 'var(--font-size-sm)',
                    color: 'var(--color-text)'
                  }}
                  onMouseEnter={e => { if (formData.modelName !== m.id) e.currentTarget.style.background = 'var(--color-bg-secondary)' }}
                  onMouseLeave={e => { if (formData.modelName !== m.id) e.currentTarget.style.background = 'none' }}
                >
                  <code style={{ fontSize: '0.8rem' }}>{m.id}</code>
                  {m.name && m.name !== m.id && (
                    <span style={{ marginLeft: '0.5rem', color: 'var(--color-text-muted)' }}>— {m.name}</span>
                  )}
                </button>
              ))}
            </div>
          )}
          <small>The actual model identifier used in API calls (e.g., "gpt-4o", "claude-3-5-sonnet")</small>
        </div>

        <div className="form-group">
          <label htmlFor="id">Model ID *</label>
          <input
            id="id"
            type="text"
            required
            value={formData.id}
            onChange={e => setFormData({ ...formData, id: e.target.value })}
            placeholder="gpt-4o"
            disabled={!!model}
          />
          <small>
            {model ? 'Model ID cannot be changed' : 'Unique identifier (lowercase, no spaces)'}
          </small>
        </div>

        <div className="form-group">
          <label htmlFor="name">Name *</label>
          <input
            id="name"
            type="text"
            required
            value={formData.name}
            onChange={e => setFormData({ ...formData, name: e.target.value })}
            placeholder="GPT-4o"
          />
          <small>Human-readable display name</small>
        </div>

        <div className="form-group">
          <label htmlFor="provider">Provider *</label>
          <input
            id="provider"
            type="text"
            required
            value={formData.provider}
            onChange={e => setFormData({ ...formData, provider: e.target.value })}
            placeholder="openai"
          />
          <small>Provider name (auto-filled from auth config)</small>
        </div>

        <div className="form-group">
          <label htmlFor="baseUrl">Endpoint URL (Optional)</label>
          <input
            id="baseUrl"
            type="text"
            value={formData.baseUrl}
            onChange={e => setFormData({ ...formData, baseUrl: e.target.value })}
            placeholder="https://api.openai.com/v1"
          />
          <small>Override the default API endpoint URL</small>
        </div>

        <div className="form-group">
          <label htmlFor="defaultParams">Default Params (JSON)</label>
          <textarea
            id="defaultParams"
            rows={5}
            value={defaultParams}
            onChange={e => setDefaultParams(e.target.value)}
            placeholder={DEFAULT_PARAMS_EXAMPLE}
            style={{ fontFamily: 'monospace', fontSize: '0.9em' }}
          />
          <small>Default parameters like temperature and max_tokens (JSON format)</small>
        </div>

        <div className="form-group">
          <label style={{ display: 'flex', alignItems: 'center', cursor: 'pointer' }}>
            <input
              type="checkbox"
              checked={setCopilotDefault}
              onChange={e => setSetCopilotDefault(e.target.checked)}
              style={{ marginRight: '0.5rem' }}
            />
            <span>Set as default model for CopilotKit</span>
          </label>
          <small style={{ marginLeft: '1.5rem' }}>
            Use this model by default in the CopilotKit chat assistant
          </small>
        </div>
      </form>
    </Modal>
  )
}
