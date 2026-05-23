import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useAuthStore } from '../stores/auth'
import { apiGet } from '../api/client'

export const Route = createFileRoute('/usage')({
  component: UsagePage,
})

interface UsageStats {
  total_calls: number
  total_tokens: number
  total_credits: number
  avg_latency_ms: number
}

interface UsageRecord {
  id: number
  model: string
  tokens: number
  credits: number
  latency_ms: number
  created_at: string
}

function UsagePage() {
  const token = useAuthStore((s) => s.token)
  const [stats, setStats] = useState<UsageStats | null>(null)
  const [records, setRecords] = useState<UsageRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [page, setPage] = useState(1)
  const pageSize = 20

  useEffect(() => {
    if (!token) return
    setLoading(true)
    Promise.all([
      apiGet<UsageStats>('/usage/stats', token),
      apiGet<UsageRecord[]>(`/usage/records?page=${page}&page_size=${pageSize}`, token),
    ])
      .then(([s, r]) => {
        setStats(s)
        setRecords(r)
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }, [token, page])

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-2xl font-bold">Usage</h1>

      {/* Summary cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <div className="border rounded-lg p-4">
          <p className="text-sm text-muted-foreground">Total Calls</p>
          <p className="text-2xl font-bold">{loading ? '...' : stats?.total_calls?.toLocaleString() ?? '0'}</p>
        </div>
        <div className="border rounded-lg p-4">
          <p className="text-sm text-muted-foreground">Total Tokens</p>
          <p className="text-2xl font-bold">{loading ? '...' : stats?.total_tokens?.toLocaleString() ?? '0'}</p>
        </div>
        <div className="border rounded-lg p-4">
          <p className="text-sm text-muted-foreground">Total Credits</p>
          <p className="text-2xl font-bold">{loading ? '...' : (stats?.total_credits ?? 0).toFixed(2)}</p>
        </div>
        <div className="border rounded-lg p-4">
          <p className="text-sm text-muted-foreground">Avg Latency</p>
          <p className="text-2xl font-bold">{loading ? '...' : (stats?.avg_latency_ms ?? 0).toFixed(0)} ms</p>
        </div>
      </div>

      {/* Usage records table */}
      <div>
        <h2 className="font-semibold mb-3">Usage Records</h2>
        {loading && <p className="text-muted-foreground">Loading...</p>}
        {error && <p className="text-red-500 text-sm">{error}</p>}
        {!loading && !error && records.length === 0 && (
          <p className="text-muted-foreground">No usage records yet.</p>
        )}
        {records.length > 0 && (
          <div className="border rounded-md overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-muted/50">
                <tr>
                  <th className="text-left p-3 font-medium">Model</th>
                  <th className="text-right p-3 font-medium">Tokens</th>
                  <th className="text-right p-3 font-medium">Credits</th>
                  <th className="text-right p-3 font-medium">Latency</th>
                  <th className="text-left p-3 font-medium">Timestamp</th>
                </tr>
              </thead>
              <tbody>
                {records.map((r) => (
                  <tr key={r.id} className="border-t">
                    <td className="p-3">{r.model}</td>
                    <td className="p-3 text-right">{r.tokens.toLocaleString()}</td>
                    <td className="p-3 text-right">{r.credits.toFixed(4)}</td>
                    <td className="p-3 text-right">{r.latency_ms}ms</td>
                    <td className="p-3 text-muted-foreground">{new Date(r.created_at).toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {records.length > 0 && (
          <div className="flex justify-center gap-2 mt-4">
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page === 1}
              className="px-3 py-1 text-sm border rounded-md disabled:opacity-50"
            >
              Prev
            </button>
            <span className="px-3 py-1 text-sm text-muted-foreground">Page {page}</span>
            <button
              onClick={() => setPage((p) => p + 1)}
              disabled={records.length < pageSize}
              className="px-3 py-1 text-sm border rounded-md disabled:opacity-50"
            >
              Next
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
