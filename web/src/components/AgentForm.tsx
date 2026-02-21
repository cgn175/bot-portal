import { useState, useEffect } from 'react'
import { api, CreateAgentRequest, Agent } from '../api/client'

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
    description: ''
  })
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (agent) {
      setFormData({
        id: agent.id,
        name: agent.name,
        image: agent.image,
        agentType: agent.agentType,
        endpoint: agent.endpoint,
        description: agent.description || ''
      })
    }
  }, [agent])

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

  return (
    <div className="modal-overlay" onClick={onCancel}>
      <div className="modal" onClick={e => e.stopPropagation()}>
        <h3 style={{ fontSize: '1.5rem', fontWeight: '600', marginBottom: '1.5rem' }}>
          {agent ? 'Edit Agent' : 'Register New Agent'}
        </h3>
        
        {error && <div className="error-message">{error}</div>}
        
        <form onSubmit={handleSubmit}>
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
            />
            <small style={{ display: 'block', marginTop: '0.25rem', color: 'var(--color-text-muted)', fontSize: '0.75rem' }}>
              {agent ? 'Agent ID cannot be changed' : 'Unique identifier for this agent (lowercase, no spaces)'}
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
            <small style={{ display: 'block', marginTop: '0.25rem', color: 'var(--color-text-muted)', fontSize: '0.75rem' }}>
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
            <small style={{ display: 'block', marginTop: '0.25rem', color: 'var(--color-text-muted)', fontSize: '0.75rem' }}>
              Human-readable display name
            </small>
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
            <small style={{ display: 'block', marginTop: '0.25rem', color: 'var(--color-text-muted)', fontSize: '0.75rem' }}>
              Optional description of the agent's purpose and capabilities
            </small>
          </div>

          <div className="form-group">
            <label htmlFor="image">Docker Image {formData.agentType === 'docker' ? '*' : ''}</label>
            <input
              id="image"
              type="text"
              required={formData.agentType === 'docker'}
              value={formData.image}
              onChange={e => setFormData({ ...formData, image: e.target.value })}
              placeholder="my-agent:latest"
              disabled={formData.agentType === 'native'}
            />
            <small style={{ display: 'block', marginTop: '0.25rem', color: 'var(--color-text-muted)', fontSize: '0.75rem' }}>
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
            <small style={{ display: 'block', marginTop: '0.25rem', color: 'var(--color-text-muted)', fontSize: '0.75rem' }}>
              Internal endpoint URL (use container name for Docker network)
            </small>
          </div>

          <div className="form-actions">
            <button type="button" className="btn btn-secondary" onClick={onCancel} disabled={loading}>
              Cancel
            </button>
            <button type="submit" className="btn btn-primary" disabled={loading}>
              {loading ? (agent ? 'Updating...' : 'Creating...') : (agent ? 'Update Agent' : 'Create Agent')}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
