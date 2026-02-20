import { useState, useEffect } from 'react'
import { useParams } from 'react-router-dom'

interface Agent {
  id: string
  name: string
  description: string
  status: string
  endpoint: string
  image: string
}

export default function AgentDetail() {
  const { id } = useParams<{ id: string }>()
  const [agent, setAgent] = useState<Agent | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (id) {
      fetch(`/api/agents/${id}`)
        .then(res => res.json())
        .then(data => {
          setAgent(data)
          setLoading(false)
        })
        .catch(() => setLoading(false))
    }
  }, [id])

  const handleAction = async (action: string) => {
    if (!id) return
    await fetch(`/api/agents/${id}?action=${action}`, { method: 'POST' })
    // Refresh agent data
    if (id) {
      const res = await fetch(`/api/agents/${id}`)
      const data = await res.json()
      setAgent(data)
    }
  }

  if (loading) return <div>Loading...</div>
  if (!agent) return <div>Agent not found</div>

  return (
    <div className="agent-detail">
      <h2>{agent.name}</h2>
      <p className="description">{agent.description}</p>
      <p className={`status ${agent.status}`}>Status: {agent.status}</p>
      <p className="endpoint">Endpoint: {agent.endpoint}</p>
      <p className="image">Image: {agent.image}</p>

      <div className="actions">
        <button onClick={() => handleAction('start')}>Start</button>
        <button onClick={() => handleAction('stop')}>Stop</button>
        <button onClick={() => handleAction('restart')}>Restart</button>
      </div>
    </div>
  )
}
