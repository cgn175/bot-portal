import { useState, useEffect, useCallback } from 'react'
import { api, AgentIdentityFileInfo } from '../api/client'

interface UseIdentityFilesResult {
  files: AgentIdentityFileInfo[]
  loading: boolean
  error: string | null
  refresh: () => Promise<void>
}

export function useIdentityFiles(agentId: string | undefined): UseIdentityFilesResult {
  const [files, setFiles] = useState<AgentIdentityFileInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    if (!agentId) {
      setFiles([])
      return
    }

    try {
      setLoading(true)
      setError(null)
      const response = await api.getAgentIdentityFiles(agentId)
      setFiles(response.files || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load identity files')
    } finally {
      setLoading(false)
    }
  }, [agentId])

  useEffect(() => {
    refresh()
  }, [refresh])

  return { files, loading, error, refresh }
}

interface UseIdentityFileResult {
  content: string
  loading: boolean
  saving: boolean
  error: string | null
  load: () => Promise<void>
  save: (content: string) => Promise<void>
}

export function useIdentityFile(
  agentId: string | undefined,
  filename: string | undefined
): UseIdentityFileResult {
  const [content, setContent] = useState('')
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    if (!agentId || !filename) {
      setContent('')
      return
    }

    try {
      setLoading(true)
      setError(null)
      const response = await api.getIdentityFile(agentId, filename)
      setContent(response.content || '')
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to load ${filename}`)
    } finally {
      setLoading(false)
    }
  }, [agentId, filename])

  const save = useCallback(async (newContent: string) => {
    if (!agentId || !filename) {
      throw new Error('Agent ID and filename are required')
    }

    try {
      setSaving(true)
      setError(null)
      await api.updateIdentityFile(agentId, filename, newContent)
      setContent(newContent)
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to save ${filename}`)
      throw err
    } finally {
      setSaving(false)
    }
  }, [agentId, filename])

  useEffect(() => {
    load()
  }, [load])

  return { content, loading, saving, error, load, save }
}
