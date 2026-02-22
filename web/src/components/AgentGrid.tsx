import { memo } from 'react'
import { Agent } from '../api/client'
import AgentCard from './AgentCard'
import EmptyState from './EmptyState'

interface AgentGridProps {
  agents: Agent[]
  onDelete: (id: string) => void
  onAction: (id: string, action: 'start' | 'stop' | 'restart') => void
}

function AgentGrid({ agents, onDelete, onAction }: AgentGridProps) {
  if (agents.length === 0) {
    return (
      <EmptyState
        icon="🤖"
        title="No agents found"
        description="Get started by registering your first agent to manage your AI workflows."
      />
    )
  }

  return (
    <div className="grid grid-auto">
      {agents.map((agent, index) => (
        <div
          key={agent.id}
          className={`animate-fade-in stagger-${Math.min(index + 1, 5)}`}
          style={{ animationDelay: `${index * 0.05}s` }}
        >
          <AgentCard
            agent={agent}
            onDelete={onDelete}
            onAction={onAction}
          />
        </div>
      ))}
    </div>
  )
}

export default memo(AgentGrid)
