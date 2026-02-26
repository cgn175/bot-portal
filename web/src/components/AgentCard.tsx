import { memo } from 'react'
import { Link } from 'react-router-dom'
import { Agent } from '../api/client'
import StatusBadge from './StatusBadge'

interface AgentCardProps {
  agent: Agent
  onDelete: (id: string) => void
  onAction: (id: string, action: 'start' | 'stop' | 'restart') => void
}

function AgentCard({ agent, onDelete, onAction }: AgentCardProps) {
  const isDocker = agent.agentType === 'docker'

  return (
    <div className="card animate-fade-in">
      <div className="card-header">
        <div>
          <h3 className="card-title">{agent.name}</h3>
          <div className="card-subtitle" style={{ display: 'flex', gap: '0.5rem', marginTop: '0.5rem' }}>
            <StatusBadge status={agent.status} />
            <StatusBadge status={agent.agentType} />
          </div>
        </div>
      </div>

      {agent.description && (
        <p style={{ color: 'var(--color-text-secondary)', fontSize: '0.875rem', marginBottom: '1rem' }}>
          {agent.description}
        </p>
      )}

      <div style={{ fontSize: '0.875rem', color: 'var(--color-text-muted)', marginBottom: '1.5rem' }}>
        <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.5rem' }}>
          <span style={{ color: 'var(--color-text-subtle)', minWidth: '70px' }}>ID:</span>
          <code>{agent.id}</code>
        </div>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <span style={{ color: 'var(--color-text-subtle)', minWidth: '70px' }}>Image:</span>
          <code style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {agent.image || 'N/A'}
          </code>
        </div>
      </div>

      <div className="card-footer" style={{ marginTop: 0, paddingTop: '1rem' }}>
        <Link
          to={`/agents/${agent.id}`}
          className="btn btn-secondary"
          style={{ textDecoration: 'none', flex: 1 }}
        >
          Details
        </Link>

        {isDocker && agent.status === 'stopped' && (
          <button
            className="btn btn-primary"
            onClick={() => onAction(agent.id, 'start')}
            title="Start agent"
          >
            Start
          </button>
        )}

        {isDocker && agent.status === 'running' && (
          <>
            <button
              className="btn btn-secondary"
              onClick={() => onAction(agent.id, 'restart')}
              title="Restart agent"
            >
              Restart
            </button>
            <button
              className="btn btn-secondary"
              onClick={() => onAction(agent.id, 'stop')}
              title="Stop agent"
            >
              Stop
            </button>
          </>
        )}

        <button
          className="btn btn-danger"
          onClick={() => onDelete(agent.id)}
          title="Delete agent"
        >
          Delete
        </button>
      </div>
    </div>
  )
}

export default memo(AgentCard, (prev, next) => {
  return (
    prev.agent.id === next.agent.id &&
    prev.agent.status === next.agent.status &&
    prev.agent.name === next.agent.name &&
    prev.agent.description === next.agent.description
  )
})
