import { useState, useRef, useEffect } from 'react'

export interface SelectOption {
  value: string
  label: string
}

interface SearchableSelectProps {
  options: SelectOption[]
  value: string
  onChange: (value: string) => void
  placeholder?: string
  disabled?: boolean
  id?: string
}

export default function SearchableSelect({
  options,
  value,
  onChange,
  placeholder = 'Select...',
  disabled = false,
  id
}: SearchableSelectProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [searchTerm, setSearchTerm] = useState('')
  const dropdownRef = useRef<HTMLDivElement>(null)

  // Find the currently selected option to display it
  const selectedOption = options.find(opt => opt.value === value)

  // Filter options based on search term
  const filteredOptions = options.filter(opt =>
    opt.label.toLowerCase().includes(searchTerm.toLowerCase())
  )

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false)
      }
    }

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside)
    }
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [isOpen])

  // Reset search term when dropdown opens
  useEffect(() => {
    if (isOpen) {
      setSearchTerm('')
    }
  }, [isOpen])

  const handleSelect = (selectedValue: string) => {
    onChange(selectedValue)
    setIsOpen(false)
  }

  return (
    <div 
      className={`searchable-select ${disabled ? 'disabled' : ''}`} 
      ref={dropdownRef}
      style={{ position: 'relative', width: '100%' }}
      id={id ? `${id}-container` : undefined}
    >
      <div
        className="select-trigger"
        onClick={() => !disabled && setIsOpen(!isOpen)}
        style={{
          padding: '0.5rem 1rem',
          borderRadius: '6px',
          border: '1px solid var(--color-border)',
          background: 'var(--color-bg-secondary, var(--color-bg))',
          color: disabled ? 'var(--color-text-muted)' : 'var(--color-text)',
          cursor: disabled ? 'not-allowed' : 'pointer',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          minHeight: '38px',
          fontSize: '0.9rem'
        }}
      >
        <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {selectedOption ? selectedOption.label : placeholder}
        </span>
        <span style={{ paddingLeft: '0.5rem', opacity: 0.5 }}>▼</span>
      </div>

      {isOpen && (
        <div
          className="select-dropdown"
          style={{
            position: 'absolute',
            top: '100%',
            left: 0,
            right: 0,
            marginTop: '4px',
            background: 'var(--color-bg-secondary, var(--color-bg))',
            border: '1px solid var(--color-border)',
            borderRadius: '6px',
            boxShadow: '0 4px 12px rgba(0, 0, 0, 0.1)',
            zIndex: 1000,
            maxHeight: '250px',
            display: 'flex',
            flexDirection: 'column'
          }}
        >
          <div style={{ padding: '0.5rem', borderBottom: '1px solid var(--color-border)' }}>
            <input
              type="text"
              autoFocus
              placeholder="Search..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              onClick={(e) => e.stopPropagation()}
              style={{
                width: '100%',
                padding: '0.5rem',
                borderRadius: '4px',
                border: '1px solid var(--color-border)',
                background: 'var(--color-bg)',
                color: 'var(--color-text)',
                fontSize: '0.9rem'
              }}
            />
          </div>
          
          <div style={{ overflowY: 'auto' }}>
            {filteredOptions.length === 0 ? (
              <div style={{ padding: '0.75rem 1rem', color: 'var(--color-text-muted)', fontSize: '0.9rem' }}>
                No options found
              </div>
            ) : (
              filteredOptions.map((opt) => (
                <div
                  key={opt.value}
                  className="select-option"
                  onClick={() => handleSelect(opt.value)}
                  style={{
                    padding: '0.75rem 1rem',
                    cursor: 'pointer',
                    fontSize: '0.9rem',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    background: value === opt.value ? 'var(--color-bg-hover, rgba(255, 255, 255, 0.05))' : 'transparent',
                    color: value === opt.value ? 'var(--color-primary)' : 'var(--color-text)'
                  }}
                  onMouseEnter={(e) => {
                    if (value !== opt.value) {
                      e.currentTarget.style.background = 'var(--color-bg-hover, rgba(255, 255, 255, 0.05))'
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (value !== opt.value) {
                      e.currentTarget.style.background = 'transparent'
                    }
                  }}
                >
                  <span style={{ 
                    overflow: 'hidden', 
                    textOverflow: 'ellipsis', 
                    whiteSpace: 'nowrap',
                    fontWeight: value === opt.value ? 500 : 400
                  }}>
                    {opt.label}
                  </span>
                  {value === opt.value && <span>✓</span>}
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  )
}
