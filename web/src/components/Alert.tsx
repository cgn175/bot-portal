import { memo, ReactNode } from 'react'

type AlertType = 'error' | 'success' | 'warning' | 'info'

interface AlertProps {
  type: AlertType
  children: ReactNode
  onClose?: () => void
  className?: string
}

const alertIcons: Record<AlertType, string> = {
  error: '⚠️',
  success: '✅',
  warning: '⚡',
  info: 'ℹ️'
}

function Alert({ type, children, onClose, className = '' }: AlertProps) {
  return (
    <div className={`alert alert-${type} ${className}`} role="alert">
      <span aria-hidden="true">{alertIcons[type]}</span>
      <div style={{ flex: 1 }}>{children}</div>
      {onClose && (
        <button
          onClick={onClose}
          className="btn btn-ghost btn-sm"
          aria-label="Dismiss alert"
          style={{ padding: '0.25rem', marginLeft: '0.5rem' }}
        >
          ✕
        </button>
      )}
    </div>
  )
}

export default memo(Alert)
