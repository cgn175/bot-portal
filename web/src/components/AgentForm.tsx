import { useState } from 'react'

interface AgentFormProps {
  onSuccess: () => void
  onCancel: () => void
}

export default function AgentForm({ onSuccess, onCancel }: AgentFormProps) {
  const [formData, setFormData] = useState({
    id: '',
    name: '',
    image: '',
    endpoint: ''
  })
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      const response = await fetch('/api/agents', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(formData)
      })

      if (!response.ok) {
        const errorText = await response.text()
        throw new Error(errorText || 'Failed to create agent')
      }

      onSuccess()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create agent')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="agent-form-overlay">
      <div className="agent-form">
        <h3>Register New Agent</h3>
        
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
            />
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
          </div>

          <div className="form-group">
            <label htmlFor="image">Docker Image *</label>
            <input
              id="image"
              type="text"
              required
              value={formData.image}
              onChange={e => setFormData({ ...formData, image: e.target.value })}
              placeholder="my-agent:latest"
            />
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
          </div>

          <div className="form-actions">
            <button type="button" onClick={onCancel} disabled={loading}>
              Cancel
            </button>
            <button type="submit" disabled={loading}>
              {loading ? 'Creating...' : 'Create Agent'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
