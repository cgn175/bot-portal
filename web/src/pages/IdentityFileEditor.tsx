import { useState, useCallback, useMemo } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useIdentityFile } from '../hooks/useIdentityFiles'
import Alert from '../components/Alert'
import { LoadingState } from '../components/LoadingState'

const ALLOWED_FILES = [
  { name: 'IDENTITY.md', label: 'Identity', description: 'Core personality and behavioral guidelines' },
  { name: 'SOUL.md', label: 'Soul', description: 'Values, principles, and emotional traits' },
  { name: 'AGENTS.md', label: 'Agents', description: 'Relationships with other agents' },
  { name: 'USER.md', label: 'User', description: 'User preferences and context' },
  { name: 'TOOLS.md', label: 'Tools', description: 'Available tools and capabilities' },
]

const MAX_CHARS = 16 * 1024 // 16KB
const WARNING_THRESHOLD = 14 * 1024 // 14KB - start warning

export default function IdentityFileEditor() {
  const { agentId } = useParams<{ agentId: string }>()
  const [selectedFile, setSelectedFile] = useState<string>('IDENTITY.md')
  const [editedContent, setEditedContent] = useState<string>('')
  const [hasChanges, setHasChanges] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [saveSuccess, setSaveSuccess] = useState(false)
  const [showPreview, setShowPreview] = useState(false)

  const {
    content,
    loading,
    saving,
    error,
    save
  } = useIdentityFile(agentId, selectedFile)

  // Update edited content when file loads
  useMemo(() => {
    setEditedContent(content)
    setHasChanges(false)
    setSaveError(null)
    setSaveSuccess(false)
  }, [content, selectedFile])

  const charCount = editedContent.length
  const isOverLimit = charCount > MAX_CHARS
  const isNearLimit = charCount > WARNING_THRESHOLD && !isOverLimit

  const handleContentChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setEditedContent(e.target.value)
    setHasChanges(true)
    setSaveSuccess(false)
  }, [])

  const handleSave = useCallback(async () => {
    if (!agentId || !selectedFile || isOverLimit) return

    setSaveError(null)
    setSaveSuccess(false)

    try {
      await save(editedContent)
      setHasChanges(false)
      setSaveSuccess(true)
      setTimeout(() => setSaveSuccess(false), 3000)
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : 'Failed to save')
    }
  }, [agentId, selectedFile, editedContent, isOverLimit, save])

  const handleFileSelect = useCallback((filename: string) => {
    if (hasChanges && !confirm('You have unsaved changes. Discard them?')) {
      return
    }
    setSelectedFile(filename)
    setShowPreview(false)
  }, [hasChanges])

  if (loading) {
    return <LoadingState message={`Loading ${selectedFile}...`} />
  }

  return (
    <div>
      <div style={{ marginBottom: '1.5rem' }}>
        <Link
          to={`/agents/${agentId}`}
          className="btn btn-ghost"
          style={{ textDecoration: 'none', paddingLeft: 0 }}
        >
          ← Back to Agent
        </Link>
      </div>

      <h2 style={{
        fontSize: 'var(--font-size-3xl)',
        fontWeight: 'var(--font-weight-bold)',
        marginBottom: '0.5rem'
      }}>
        Edit Identity Files
      </h2>
      <p style={{
        color: 'var(--color-text-secondary)',
        marginBottom: '1.5rem'
      }}>
        Customize agent behavior by editing markdown files. Changes are applied immediately without restarting.
      </p>

      {error && (
        <Alert type="error" onClose={() => {}}>
          {error}
        </Alert>
      )}

      {saveError && (
        <Alert type="error" onClose={() => setSaveError(null)}>
          {saveError}
        </Alert>
      )}

      {saveSuccess && (
        <Alert type="success" onClose={() => setSaveSuccess(false)}>
          File saved successfully! Changes are now active.
        </Alert>
      )}

      <div style={{ display: 'grid', gridTemplateColumns: '250px 1fr', gap: '1.5rem' }}>
        {/* File Selector Sidebar */}
        <div className="card" style={{ padding: '1rem' }}>
          <h3 style={{
            fontSize: 'var(--font-size-sm)',
            fontWeight: 'var(--font-weight-semibold)',
            textTransform: 'uppercase',
            color: 'var(--color-text-muted)',
            marginBottom: '1rem',
            letterSpacing: '0.05em'
          }}>
            Files
          </h3>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
            {ALLOWED_FILES.map((file) => (
              <button
                key={file.name}
                onClick={() => handleFileSelect(file.name)}
                style={{
                  padding: '0.75rem 1rem',
                  borderRadius: 'var(--radius-md)',
                  border: 'none',
                  background: selectedFile === file.name
                    ? 'var(--color-primary)'
                    : 'transparent',
                  color: selectedFile === file.name
                    ? 'white'
                    : 'var(--color-text)',
                  cursor: 'pointer',
                  textAlign: 'left',
                  transition: 'all 0.2s',
                  fontSize: 'var(--font-size-sm)',
                  fontWeight: selectedFile === file.name
                    ? 'var(--font-weight-semibold)'
                    : 'var(--font-weight-normal)',
                }}
                title={file.description}
              >
                <div>{file.label}</div>
                <div style={{
                  fontSize: 'var(--font-size-xs)',
                  opacity: 0.7,
                  marginTop: '0.25rem',
                  fontWeight: 'normal'
                }}>
                  {file.name}
                </div>
              </button>
            ))}
          </div>
        </div>

        {/* Editor Area */}
        <div>
          <div className="card" style={{ marginBottom: '1rem' }}>
            {/* Toolbar */}
            <div style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              padding: '0.75rem 1rem',
              borderBottom: '1px solid var(--color-border)',
              flexWrap: 'wrap',
              gap: '0.5rem'
            }}>
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                <button
                  className={`btn btn-sm ${!showPreview ? 'btn-primary' : 'btn-secondary'}`}
                  onClick={() => setShowPreview(false)}
                >
                  Edit
                </button>
                <button
                  className={`btn btn-sm ${showPreview ? 'btn-primary' : 'btn-secondary'}`}
                  onClick={() => setShowPreview(true)}
                >
                  Preview
                </button>
              </div>

              <div style={{
                display: 'flex',
                alignItems: 'center',
                gap: '1rem',
                fontSize: 'var(--font-size-sm)'
              }}>
                <span style={{
                  color: isOverLimit
                    ? 'var(--color-danger)'
                    : isNearLimit
                      ? 'var(--color-warning)'
                      : 'var(--color-text-muted)'
                }}>
                  {charCount.toLocaleString()} / {MAX_CHARS.toLocaleString()} chars
                  {isNearLimit && ' ⚠️'}
                  {isOverLimit && ' ❌'}
                </span>
                <button
                  className="btn btn-primary btn-sm"
                  onClick={handleSave}
                  disabled={saving || !hasChanges || isOverLimit}
                >
                  {saving ? 'Saving...' : 'Save Changes'}
                </button>
              </div>
            </div>

            {/* Editor or Preview */}
            {!showPreview ? (
              <textarea
                value={editedContent}
                onChange={handleContentChange}
                style={{
                  width: '100%',
                  minHeight: '500px',
                  padding: '1rem',
                  border: 'none',
                  outline: 'none',
                  fontFamily: 'var(--font-mono, monospace)',
                  fontSize: 'var(--font-size-sm)',
                  lineHeight: '1.6',
                  resize: 'vertical',
                  background: isOverLimit ? 'rgba(239, 68, 68, 0.05)' : undefined
                }}
                placeholder={`# Enter ${selectedFile} content here...`}
                spellCheck={false}
              />
            ) : (
              <div style={{
                minHeight: '500px',
                padding: '1rem',
                overflow: 'auto'
              }}>
                <div
                  className="markdown-preview"
                  style={{
                    fontSize: 'var(--font-size-base)',
                    lineHeight: '1.6'
                  }}
                  dangerouslySetInnerHTML={{
                    __html: renderMarkdown(editedContent || '*No content*')
                  }}
                />
              </div>
            )}
          </div>

          {/* Help Text */}
          <div style={{
            fontSize: 'var(--font-size-sm)',
            color: 'var(--color-text-muted)'
          }}>
            <p style={{ marginBottom: '0.5rem' }}>
              <strong>Tip:</strong> Use Markdown formatting. Changes are saved to the database and synced to running agents automatically.
            </p>
            {isNearLimit && (
              <p style={{ color: 'var(--color-warning)' }}>
                ⚠️ Approaching size limit. Consider reducing content.
              </p>
            )}
            {isOverLimit && (
              <p style={{ color: 'var(--color-danger)' }}>
                ❌ Content exceeds 16KB limit. Please reduce before saving.
              </p>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

// Simple markdown renderer for preview
function renderMarkdown(content: string): string {
  if (!content) return ''

  return content
    // Escape HTML
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    // Headers
    .replace(/^### (.*$)/gim, '<h3>$1</h3>')
    .replace(/^## (.*$)/gim, '<h2>$1</h2>')
    .replace(/^# (.*$)/gim, '<h1>$1</h1>')
    // Bold and italic
    .replace(/\*\*\*(.*?)\*\*\*/g, '<strong><em>$1</em></strong>')
    .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.*?)\*/g, '<em>$1</em>')
    // Code blocks
    .replace(/```([\s\S]*?)```/g, '<pre><code>$1</code></pre>')
    // Inline code
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    // Lists
    .replace(/^\s*-\s+(.*$)/gim, '<li>$1</li>')
    .replace(/(<li>.*<\/li>)/s, '<ul>$1</ul>')
    // Line breaks
    .replace(/\n/g, '<br>')
}
