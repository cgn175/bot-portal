import { memo } from 'react'

interface LoadingStateProps {
  message?: string
  variant?: 'spinner' | 'pulse' | 'skeleton'
}

export function LoadingState({ message = 'Loading...', variant = 'spinner' }: LoadingStateProps) {
  return (
    <div className="loading" role="status" aria-live="polite">
      {variant === 'spinner' && <div className="loading-spinner" aria-hidden="true" />}
      {variant === 'pulse' && (
        <div className="loading-pulse" aria-hidden="true">
          <span />
          <span />
          <span />
        </div>
      )}
      {variant === 'skeleton' && (
        <div className="loading-skeleton" aria-hidden="true">
          <div className="skeleton" style={{ height: '16px', width: '120px' }} />
        </div>
      )}
      <span>{message}</span>
    </div>
  )
}

interface SkeletonCardProps {
  count?: number
}

export function SkeletonCard({ count = 3 }: SkeletonCardProps) {
  return (
    <div className="grid grid-auto" role="status" aria-label="Loading content">
      {Array.from({ length: count }).map((_, i) => (
        <div
          key={i}
          className="card"
          style={{ animationDelay: `${i * 0.1}s` }}
        >
          <div className="skeleton" style={{ height: '24px', width: '60%', marginBottom: '1rem' }} />
          <div className="skeleton" style={{ height: '16px', width: '40%', marginBottom: '1.5rem' }} />
          <div className="skeleton" style={{ height: '12px', width: '100%', marginBottom: '0.5rem' }} />
          <div className="skeleton" style={{ height: '12px', width: '80%', marginBottom: '1.5rem' }} />
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <div className="skeleton" style={{ height: '36px', flex: 1 }} />
            <div className="skeleton" style={{ height: '36px', width: '80px' }} />
          </div>
        </div>
      ))}
    </div>
  )
}

interface SkeletonTextProps {
  lines?: number
}

export function SkeletonText({ lines = 3 }: SkeletonTextProps) {
  return (
    <div role="status" aria-label="Loading content">
      {Array.from({ length: lines }).map((_, i) => (
        <div
          key={i}
          className="skeleton"
          style={{
            height: '12px',
            width: i === lines - 1 ? '60%' : '100%',
            marginBottom: i === lines - 1 ? 0 : '0.5rem'
          }}
        />
      ))}
    </div>
  )
}

export default memo(LoadingState)
