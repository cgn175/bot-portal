import { memo } from 'react'
import { Link } from 'react-router-dom'
import { Agent } from '../api/client'

interface AgentCardProps {
  agent: Agent
  onDelete: (id: string) => void
  onAction: (id: string, action: 'start' | 'stop' | 'restart') => void
}

function AgentCard({ agent, onDelete, onAction }: AgentCardProps) {
  return (
    <div className="card">
      <div style={{ marginBottom: '1rem' }}>
        <h3 style={{ fontSize: '1.25rem', fontWeight: '600', marginBottom: '0.5rem' }}>
          {agent.name}
        </h3>
        <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
          <span className={`badge ${agent.status}`}>{agent.status}</span>
          <span className="badge" style={{ background: agent.agentType === 'docker' ? 'var(--color-primary)' : 'var(--color-warning)' }}>
            {agent.agentType}
          </span>
        </div>
      </div>
      
      {agent.description && (
        <p style={{ color: 'var(--color-text-secondary)', fontSize: '0.875rem', marginBottom: '1rem' }}>
          {agent.description}
        </p>
      )}
      
      <div style={{ fontSize: '0.875rem', color: 'var(--color-text-muted)', marginBottom: '1rem' }}>
        <div style={{ marginBottom: '0.25rem' }}>
          <strong>ID:</strong> <code>{agent.id}</code>
        </div>
        <div style={{ marginBottom: '0.25rem' }}>
          <strong>Endpoint:</strong> <code>{agent.endpoint}</code>
        </div>
        <div>
          <strong>Image:</strong> <code>{agent.image}</code>
        </div>
      </div>

      <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
        <Link to={`/agents/${agent.id}`} className="btn btn-secondary" style={{ textDecoration: 'none', flex: 1 }}>
          Details
        </Link>
        {agent.agentType === 'docker' && (
          <>
            {agent.status === 'stopped' && (
              <button className="btn btn-primary" onClick={() => onAction(agent.id, 'start')}>
                Start
              </button>
            )}
            {agent.status === 'running' && (
              <>
                <button className="btn btn-secondary" onClick={() => onAction(agent.id, 'restart')}>
                  Restart
                </button>
                <button className="btn btn-secondary" onClick={() => onAction(agent.id, 'stop')}>
                  Stop
                </button>
              </>
            )}
          </>
        )}
        <button className="btn btn-danger" onClick={() => onDelete(agent.id)}>
          Delete
        </button>
      </div>
    </div>
  )
}

export default memo(AgentCard, (prev, next) => {
  return prev.agent.id === next.agent.id &&
         prev.agent.status === next.agent.status &&
         prev.agent.name === next.agent.name
})
