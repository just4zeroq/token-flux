import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useAuthStore } from '../stores/auth'
import { apiGet } from '../api/client'

export const Route = createFileRoute('/models')({
  component: ModelsPage,
})

interface CatalogItem {
  id: number
  type: string
  name: string
  description: string
  status: string
  created_at: string
}

function ModelsPage() {
  const token = useAuthStore((s) => s.token)
  const [items, setItems] = useState<CatalogItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!token) return
    apiGet<CatalogItem[]>('/catalog/items', token)
      .then(setItems)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }, [token])

  return (
    <div className="p-6 space-y-4">
      <h1 className="text-2xl font-bold">Model Catalog</h1>

      {loading && <p className="text-muted-foreground">Loading...</p>}
      {error && <p className="text-red-500 text-sm">{error}</p>}

      {!loading && !error && items.length === 0 && (
        <p className="text-muted-foreground">No models available.</p>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {items.map((item) => (
          <div key={item.id} className="border rounded-lg p-4 space-y-2">
            <div className="flex items-center justify-between">
              <h3 className="font-semibold">{item.name}</h3>
              <span className="text-xs px-2 py-1 rounded bg-blue-100 text-blue-700 capitalize">
                {item.type}
              </span>
            </div>
            <p className="text-sm text-muted-foreground">{item.description}</p>
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <span className={`px-2 py-0.5 rounded ${item.status === 'active' ? 'bg-green-100 text-green-700' : 'bg-gray-100'}`}>
                {item.status}
              </span>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
