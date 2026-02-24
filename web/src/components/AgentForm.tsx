import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { api, CreateAgentRequest, Agent, Model, AuthConfig } from '../api/client'
import Modal from './Modal'
import Alert from './Alert'

interface AgentFormProps {
  agent?: Agent
  onSuccess: () => void
  onCancel: () => void
}

export default function AgentForm({ agent, onSuccess, onCancel }: AgentFormProps) {
  const [formData, setFormData] = useState<CreateAgentRequest>({
    id: '',
    name: '',
    image: '',
    agentType: 'docker',
    endpoint: '',
    description: '',
    modelId: '',
    authConfigId: ''
  })
  const [models, setModels] = useState<Model[]>([])
  const [authConfigs, setAuthConfigs] = useState<AuthConfig[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [fetchingDeps, setFetchingDeps] = useState(true)

  useEffect(() => {
    if (agent) {
      setFormData({
        id: agent.id,
        name: agent.name,
        image: agent.image,
        agentType: agent.agentType,
        endpoint: agent.endpoint,
        description: agent.description || '',
        modelId: agent.modelId || '',
        authConfigId: agent.authConfigId || ''
      })
    }
  }, [agent])

  useEffect(() => {
    // Load available models and auth configs
    const loadDependencies = async () => {
      try {
        const [modelsData, authConfigsData] = await Promise.all([
          api.listModels(),
          api.listAuthConfigs()
        ])
        setModels(modelsData)
        setAuthConfigs(authConfigsData)
      } catch (err) {
        console.error('Failed to load form dependencies', err)
      } finally {
        setFetchingDeps(false)
      }
    }
    loadDependencies()
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      if (agent) {
        await api.updateAgent(agent.id, formData)
      } else {
        await api.createAgent(formData)
      }
      onSuccess()
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to ${agent ? 'update' : 'create'} agent`)
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
        form="agent-form"
      >
        {loading ? (agent ? 'Updating...' : 'Creating...') : (agent ? 'Update Agent' : 'Create Agent')}
      </button>
    </>
  )

  if (fetchingDeps) {
    return (
      <Modal isOpen={true} onClose={onCancel} title={agent ? 'Edit Agent' : 'Register New Agent'} size="md">
        <div style={{ padding: '3rem 2rem', textAlign: 'center', color: 'var(--color-text-muted)' }}>
          Loading metadata...
        </div>
      </Modal>
    )
  }

  if (authConfigs.length === 0 && !agent) {
    return (
      <Modal isOpen={true} onClose={onCancel} title="Auth Provider Required" size="md">
        <div style={{ padding: '2rem', textAlign: 'center' }}>
          <div style={{ fontSize: '3rem', marginBottom: '1rem' }}>🔐</div>
          <h3 style={{ marginBottom: '1rem', color: 'var(--color-heading)' }}>No Auth Providers Found</h3>
          <p style={{ color: 'var(--color-text-secondary)', marginBottom: '1.5rem', lineHeight: '1.5' }}>
            Set up an auth provider first — models will be auto-discovered and available for your agent.
          </p>
          <div style={{ display: 'flex', gap: '0.75rem', justifyContent: 'center' }}>
            <button className="btn btn-secondary" onClick={onCancel}>
              Cancel
            </button>
            <Link to="/auth-configs" className="btn btn-primary" onClick={onCancel} style={{ textDecoration: 'none' }}>
              Add Auth Provider
            </Link>
          </div>
        </div>
      </Modal>
    )
  }

  // Filter models by selected auth config
  const selectedConfig = authConfigs.find(c => c.id === formData.authConfigId)
  const filteredModels = selectedConfig
    ? models.filter(m => {
        // Match by endpoint URL (most reliable — both set during auto-discovery)
        if (selectedConfig.endpointUrl && m.endpointUrl) {
          return m.endpointUrl === selectedConfig.endpointUrl
        }
        // Fallback: match by provider
        const configProvider = selectedConfig.authType === 'github_copilot_oauth' ? 'copilot' : selectedConfig.provider
        return m.provider === configProvider
      })
    : models

  return (
    <Modal
      isOpen={true}
      onClose={onCancel}
      title={agent ? 'Edit Agent' : 'Register New Agent'}
      footer={footer}
      size="md"
    >
      {error && (
        <Alert type="error" onClose={() => setError('')}>
          {error}
        </Alert>
      )}

      <form id="agent-form" onSubmit={handleSubmit}>
        <div className="form-group">
          <label htmlFor="id">Agent ID *</label>
          <input
            id="id"
            type="text"
            required
            value={formData.id}
            onChange={e => setFormData({ ...formData, id: e.target.value })}
            placeholder="agent1"
            disabled={!!agent}
            className={error ? 'error' : ''}
          />
          <small>
            {agent ? 'Agent ID cannot be changed' : 'Unique identifier (lowercase, no spaces)'}
          </small>
        </div>

        <div className="form-group">
          <label htmlFor="agentType">Agent Type *</label>
          <select
            id="agentType"
            required
            value={formData.agentType}
            onChange={e => setFormData({ ...formData, agentType: e.target.value as 'docker' | 'native' })}
          >
            <option value="docker">Docker Container</option>
            <option value="native">Native Process</option>
          </select>
          <small>
            Docker: Managed by portal. Native: External process you manage
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
            placeholder="My AI Agent"
          />
          <small>Human-readable display name</small>
        </div>

        <div className="form-group">
          <label htmlFor="description">Description</label>
          <textarea
            id="description"
            rows={3}
            value={formData.description}
            onChange={e => setFormData({ ...formData, description: e.target.value })}
            placeholder="What does this agent do?"
          />
          <small>Optional description of the agent's purpose</small>
        </div>

        <div className="form-group">
          <label htmlFor="image">
            Docker Image {formData.agentType === 'docker' ? '*' : ''}
          </label>
          <input
            id="image"
            type="text"
            required={formData.agentType === 'docker'}
            value={formData.image}
            onChange={e => setFormData({ ...formData, image: e.target.value })}
            placeholder="my-agent:latest"
            disabled={formData.agentType === 'native'}
          />
          <small>
            {formData.agentType === 'docker'
              ? 'Docker image name with tag (e.g., username/agent:v1.0)'
              : 'Not required for native agents'}
          </small>
        </div>

        <div className="form-group">
          <label htmlFor="endpoint">Endpoint *</label>
          <input
            id="endpoint"
            type="text"
            required
            value={formData.endpoint}
            onChange={e => setFormData({ ...formData, endpoint: e.target.value })}
            placeholder="http://agent1:8080"
          />
          <small>Internal endpoint URL (use container name for Docker network)</small>
        </div>

        <div className="form-group">
          <label htmlFor="authConfigId">Auth Provider *</label>
          <select
            id="authConfigId"
            required
            value={formData.authConfigId || ''}
            onChange={e => {
              const configId = e.target.value
              setFormData({ ...formData, authConfigId: configId, modelId: '' })
            }}
          >
            <option value="">— Select provider —</option>
            {authConfigs.map(c => (
              <option key={c.id} value={c.id}>
                {c.name} ({c.authType === 'github_copilot_oauth' ? 'GitHub Copilot' : 'Custom'})
              </option>
            ))}
          </select>
          <small>Credentials and API endpoint for this agent</small>
        </div>

        <div className="form-group">
          <label htmlFor="modelId">Model *</label>
          <select
            id="modelId"
            required
            value={formData.modelId || ''}
            onChange={e => setFormData({ ...formData, modelId: e.target.value })}
            disabled={!formData.authConfigId}
          >
            <option value="">{formData.authConfigId ? '— Select model —' : '— Select a provider first —'}</option>
            {filteredModels.map(m => (
              <option key={m.id} value={m.id}>
                {m.name} ({m.modelIdentifier})
              </option>
            ))}
          </select>
          <small>
            {formData.authConfigId && filteredModels.length === 0
              ? 'No models found for this provider — try syncing on the Models page'
              : 'The AI model this agent will use'}
          </small>
        </div>
      </form>
    </Modal>
  )
}
