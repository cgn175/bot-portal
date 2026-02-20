import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import AgentForm from '../components/AgentForm'

interface Agent {
  id: string
  name: string
  status: string
  endpoint: string
}

export default function Dashboard() {
  const [agents, setAgents] = useState<Agent[]>([])
  const [loading, setLoading] = useState(true)
  const [showForm, setShowForm] = useState(false)

  const loadAgents = () => {
    setLoading(true)
    fetch('/api/agents')
      .then(res => res.json())
      .then(data => {
        setAgents(data)
        setLoading(false)
      })
      .catch(() => setLoading(false))
  }

  useEffect(() => {
    loadAgents()
  }, [])

  const handleAgentCreated = () => {
    setShowForm(false)
    loadAgents()
  }

  if (loading) {
    return <div>Loading...</div>
  }

  return (
    <div className="dashboard">
      <div className="dashboard-header">
        <h2>Agents</h2>
        <button className="btn-primary" onClick={() => setShowForm(true)}>
          + Register Agent
        </button>
      </div>

      <div className="agent-grid">
        {!agents || agents.length === 0 ? (
          <p>No agents registered yet.</p>
        ) : (
          agents.map(agent => (
            <div key={agent.id} className="agent-card">
              <h3>{agent.name}</h3>
              <p className={`status ${agent.status}`}>{agent.status}</p>
              <p className="endpoint">{agent.endpoint}</p>
              <Link to={`/agents/${agent.id}`}>View Details</Link>
            </div>
          ))
        )}
      </div>

      {showForm && (
        <AgentForm
          onSuccess={handleAgentCreated}
          onCancel={() => setShowForm(false)}
        />
      )}
    </div>
  )
}
