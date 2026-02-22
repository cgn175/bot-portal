import { memo } from 'react'

type StatusType = 'running' | 'stopped' | 'error' | 'pending' | 'completed' | 'failed'
type DirectionType = 'inbound' | 'outbound'
type AgentType = 'docker' | 'native'

interface StatusBadgeProps {
  status: StatusType | DirectionType | AgentType | string
  className?: string
}

const statusConfig: Record<string, { label: string; className: string }> = {
  // Status types
  running: { label: 'Running', className: 'badge-running' },
  stopped: { label: 'Stopped', className: 'badge-stopped' },
  error: { label: 'Error', className: 'badge-error' },
  pending: { label: 'Pending', className: 'badge-pending' },
  completed: { label: 'Completed', className: 'badge-completed' },
  failed: { label: 'Failed', className: 'badge-failed' },
  // Direction types
  inbound: { label: 'Inbound', className: 'badge-inbound' },
  outbound: { label: 'Outbound', className: 'badge-outbound' },
  // Agent types
  docker: { label: 'Docker', className: 'badge-docker' },
  native: { label: 'Native', className: 'badge-native' }
}

function StatusBadge({ status, className = '' }: StatusBadgeProps) {
  const config = statusConfig[status] || { label: status, className: '' }

  return (
    <span className={`badge ${config.className} ${className}`}>
      {config.label}
    </span>
  )
}

export default memo(StatusBadge)
