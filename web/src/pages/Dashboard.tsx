import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'

interface Agent {
  id: string
  name: string
  status: string
  endpoint: string
}

export default function Dashboard() {
  const [agents, setAgents] = useState<Agent[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetch('/api/agents')
      .then(res => res.json())
      .then(data => {
        setAgents(data)
        setLoading(false)
      })
      .catch(() => setLoading(false))
  }, [])

  if (loading) {
    return <div>Loading...</div>
  }

  return (
    <div className="dashboard">
      <h2>Agents</h2>
      <div className="agent-grid">
        {agents.length === 0 ? (
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
    </div>
  )
}
