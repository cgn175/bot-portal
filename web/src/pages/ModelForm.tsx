import { useState, useEffect } from 'react'
import { api, CreateModelRequest, Model } from '../api/client'
import Modal from '../components/Modal'
import Alert from '../components/Alert'

interface ModelFormProps {
  model?: Model
  onSuccess: () => void
  onCancel: () => void
}

const PROVIDERS = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'copilot', label: 'Copilot' },
  { value: 'ollama', label: 'Ollama' },
  { value: 'openrouter', label: 'OpenRouter' }
]

const DEFAULT_PARAMS_EXAMPLE = `{
  "temperature": 0.7,
  "max_tokens": 4096
}`

export default function ModelForm({ model, onSuccess, onCancel }: ModelFormProps) {
  const [formData, setFormData] = useState<CreateModelRequest>({
    id: '',
    name: '',
    provider: 'openai',
    modelName: '',
    baseUrl: '',
    apiKeyConfig: {}
  })
  const [defaultParams, setDefaultParams] = useState(DEFAULT_PARAMS_EXAMPLE)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

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
            className={error ? 'error' : ''}
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
          <select
            id="provider"
            required
            value={formData.provider}
            onChange={e => setFormData({ ...formData, provider: e.target.value })}
          >
            {PROVIDERS.map(p => (
              <option key={p.value} value={p.value}>{p.label}</option>
            ))}
          </select>
          <small>Select the AI provider for this model</small>
        </div>

        <div className="form-group">
          <label htmlFor="modelName">Model Identifier *</label>
          <input
            id="modelName"
            type="text"
            required
            value={formData.modelName}
            onChange={e => setFormData({ ...formData, modelName: e.target.value })}
            placeholder="gpt-4o"
          />
          <small>The actual model identifier used in API calls (e.g., "gpt-4o", "claude-3-5-sonnet")</small>
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
          <small>Override the default API endpoint URL for compatible proxies</small>
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
      </form>
    </Modal>
  )
}
