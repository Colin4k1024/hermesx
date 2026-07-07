import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@shared/api/client'

export interface FileEntry {
  id: string
  tenant_id: string
  user_id: string
  path: string
  minio_key: string
  size_bytes: number
  mime_type: string
  sha256: string
  source_session?: string
  created_at: string
  updated_at: string
}

interface FileListResponse {
  files: FileEntry[]
  total: number
}

/** Fetch workspace files for the current user. */
export function useFiles() {
  return useQuery({
    queryKey: ['files'],
    queryFn: async () => {
      const resp = await apiClient.get<FileListResponse>('/v1/files')
      return resp.files ?? []
    },
  })
}

/** Upload a file to the workspace. */
export function useUploadFile() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ file, path }: { file: File; path?: string }) => {
      const formData = new FormData()
      formData.append('file', file)
      if (path) formData.append('path', path)

      const state = await import('@shared/stores/authStore').then((m) =>
        m.useAuthStore.getState(),
      )
      const key = state.userApiKey
      const headers: Record<string, string> = {}
      if (key) headers['Authorization'] = `Bearer ${key}`

      const res = await fetch('/v1/files/upload', {
        method: 'POST',
        headers,
        body: formData,
      })

      if (!res.ok) {
        const text = await res.text().catch(() => res.statusText)
        throw new Error(text || `Upload failed: ${res.status}`)
      }

      return res.json() as Promise<FileEntry>
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['files'] })
    },
  })
}

/** Delete a workspace file by ID. */
export function useDeleteFile() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) =>
      apiClient.del<{ success: boolean }>(`/v1/files/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['files'] })
    },
  })
}

/** Download a workspace file by ID. Returns a blob URL. */
export async function downloadFile(id: string): Promise<void> {
  const state = await import('@shared/stores/authStore').then((m) =>
    m.useAuthStore.getState(),
  )
  const key = state.userApiKey
  const headers: Record<string, string> = {}
  if (key) headers['Authorization'] = `Bearer ${key}`

  const res = await fetch(`/v1/files/${id}/download`, { headers })
  if (!res.ok) {
    throw new Error(`Download failed: ${res.status}`)
  }

  const blob = await res.blob()
  const disposition = res.headers.get('Content-Disposition')
  let filename = 'download'
  if (disposition) {
    const match = disposition.match(/filename="?([^"]+)"?/)
    if (match && match[1]) filename = match[1]
  }

  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
