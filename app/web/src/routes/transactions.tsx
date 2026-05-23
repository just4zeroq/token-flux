import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useAuthStore } from '../stores/auth'
import { apiGet } from '../api/client'

export const Route = createFileRoute('/transactions')({
  component: TransactionsPage,
})

interface Transaction {
  id: number
  type: string
  entries: { delta: number }[]
  created_at: string
}

interface TransactionsResponse {
  list: Transaction[]
  total: number
  page: number
  page_size: number
}

function TransactionsPage() {
  const token = useAuthStore((s) => s.token)
  const [data, setData] = useState<TransactionsResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [page, setPage] = useState(1)
  const pageSize = 20

  useEffect(() => {
    if (!token) return
    setLoading(true)
    apiGet<TransactionsResponse>(`/billing/transactions?page=${page}&page_size=${pageSize}`, token)
      .then(setData)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }, [token, page])

  const totalPages = data ? Math.ceil(data.total / data.page_size) : 1

  return (
    <div className="p-6 space-y-4">
      <h1 className="text-2xl font-bold">Transactions</h1>

      {loading && <p className="text-muted-foreground">Loading...</p>}
      {error && <p className="text-red-500 text-sm">{error}</p>}

      {!loading && !error && data && data.list.length === 0 && (
        <p className="text-muted-foreground">No transactions yet.</p>
      )}

      {data && data.list.length > 0 && (
        <>
          <div className="border rounded-md overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-muted/50">
                <tr>
                  <th className="text-left p-3 font-medium">ID</th>
                  <th className="text-left p-3 font-medium">Type</th>
                  <th className="text-right p-3 font-medium">Amount</th>
                  <th className="text-left p-3 font-medium">Timestamp</th>
                </tr>
              </thead>
              <tbody>
                {data.list.map((t) => {
                  const totalDelta = t.entries?.reduce((sum, e) => sum + e.delta, 0) ?? 0
                  return (
                    <tr key={t.id} className="border-t">
                      <td className="p-3 font-mono text-xs">{t.id}</td>
                      <td className="p-3 capitalize">{t.type}</td>
                      <td className={`p-3 text-right ${totalDelta >= 0 ? 'text-green-600' : 'text-red-600'}`}>
                        {totalDelta >= 0 ? '+' : ''}{(totalDelta / 1_000_000).toFixed(6)}
                      </td>
                      <td className="p-3 text-muted-foreground">{new Date(t.created_at).toLocaleString()}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
          <div className="flex items-center justify-center gap-2 mt-4">
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page === 1}
              className="px-3 py-1 text-sm border rounded-md disabled:opacity-50"
            >
              Prev
            </button>
            <span className="text-sm text-muted-foreground">
              Page {data.page} of {totalPages} (total {data.total})
            </span>
            <button
              onClick={() => setPage((p) => p + 1)}
              disabled={page >= totalPages}
              className="px-3 py-1 text-sm border rounded-md disabled:opacity-50"
            >
              Next
            </button>
          </div>
        </>
      )}
    </div>
  )
}
