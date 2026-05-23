import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useAuthStore } from '../stores/auth'
import { apiGet, apiPost, apiDel } from '../api/client'

export const Route = createFileRoute('/keys')({
  component: KeysPage,
})

interface ApiKey {
  id: number
  name: string
  key: string
  status: number
  created_at: string
}

function KeysPage() {
  const token = useAuthStore((s) => s.token)
  const [keys, setKeys] = useState<ApiKey[]>([])
  const [loading, setLoading] = useState(true)
  const [showNew, setShowNew] = useState(false)
  const [newName, setNewName] = useState('')
  const [newKey, setNewKey] = useState('')
  const [error, setError] = useState('')

  const fetchKeys = async () => {
    if (!token) return
    try {
      const res = await apiGet<ApiKey[]>('/users/me/keys', token)
      setKeys(res || [])
    } catch {
      // ignore
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchKeys() }, [token])

  const createKey = async () => {
    if (!token) return
    setError('')
    try {
      const res = await apiPost<{ id: number; key: string; name: string; status: number; created_at: string }>(
        '/users/me/keys', { name: newName || 'Default' }, token
      )
      setNewKey(res.key)
      setShowNew(false)
      setNewName('')
      fetchKeys()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create key')
    }
  }

  const deleteKey = async (id: number) => {
    if (!token || !confirm('Delete this API key?')) return
    try {
      await apiDel(`/users/me/keys/${id}`, token)
      fetchKeys()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete key')
    }
  }

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">API Keys</h1>
        <button
          onClick={() => setShowNew(!showNew)}
          className="bg-foreground text-background px-4 py-2 rounded-md text-sm font-medium"
        >
          Create Key
        </button>
      </div>

      {error && <p className="text-red-500 text-sm">{error}</p>}

      {newKey && (
        <div className="border border-green-500 rounded-md p-4 bg-green-50">
          <p className="text-sm font-medium text-green-800">Key created! Copy it now — it won't be shown again.</p>
          <code className="block mt-2 p-2 bg-white border rounded text-sm">{newKey}</code>
          <button onClick={() => setNewKey('')} className="mt-2 text-sm underline">Dismiss</button>
        </div>
      )}

      {showNew && (
        <div className="border rounded-md p-4 space-y-3">
          <input
            type="text"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            placeholder="Key name"
            className="w-full border rounded-md px-3 py-2 text-sm"
          />
          <div className="flex gap-2">
            <button onClick={createKey} className="bg-foreground text-background px-4 py-2 rounded-md text-sm">
              Create
            </button>
            <button onClick={() => setShowNew(false)} className="px-4 py-2 text-sm">Cancel</button>
          </div>
        </div>
      )}

      {loading ? (
        <p className="text-muted-foreground">Loading...</p>
      ) : keys.length === 0 ? (
        <p className="text-muted-foreground">No API keys yet.</p>
      ) : (
        <div className="border rounded-md">
          {keys.map((k) => (
            <div key={k.id} className="flex items-center justify-between p-3 border-b last:border-b-0">
              <div>
                <p className="font-medium text-sm">{k.name || 'Unnamed'}</p>
                <code className="text-xs text-muted-foreground">{k.key.substring(0, 12)}...</code>
              </div>
              <div className="flex items-center gap-3">
                <span className={`text-xs px-2 py-1 rounded ${k.status === 1 ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                  {k.status === 1 ? 'Active' : 'Disabled'}
                </span>
                <button onClick={() => deleteKey(k.id)} className="text-xs text-red-500 hover:underline">Delete</button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
