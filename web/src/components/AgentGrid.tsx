import { memo } from 'react'
import { Agent } from '../api/client'
import AgentCard from './AgentCard'

interface AgentGridProps {
  agents: Agent[]
  onDelete: (id: string) => void
  onAction: (id: string, action: 'start' | 'stop' | 'restart') => void
}

function AgentGrid({ agents, onDelete, onAction }: AgentGridProps) {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))', gap: '1.5rem' }}>
      {agents.map(agent => (
        <AgentCard
          key={agent.id}
          agent={agent}
          onDelete={onDelete}
          onAction={onAction}
        />
      ))}
    </div>
  )
}

export default memo(AgentGrid)
