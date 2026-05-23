import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useAuthStore } from '../stores/auth'
import { apiGet } from '../api/client'

export const Route = createFileRoute('/dashboard')({
  component: DashboardPage,
})

interface AccountInfo {
  owner_type: string
  owner_id: number
  asset: string
  balance_micro: number
}

interface UsageStats {
  total_calls: number
  total_tokens: number
  total_credits: number
  avg_latency_ms: number
}

function DashboardPage() {
  const token = useAuthStore((s) => s.token)
  const user = useAuthStore((s) => s.user)
  const [account, setAccount] = useState<AccountInfo | null>(null)
  const [stats, setStats] = useState<UsageStats | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!token) return
    Promise.all([
      apiGet<AccountInfo>('/billing/balance', token),
      apiGet<UsageStats>('/usage/stats', token),
    ])
      .then(([acc, st]) => {
        setAccount(acc)
        setStats(st)
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [token])

  const balance = account ? (account.balance_micro / 1_000_000).toFixed(2) : '0.00'

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-2xl font-bold">Dashboard</h1>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="border rounded-lg p-4">
          <p className="text-sm text-muted-foreground">Balance</p>
          <p className="text-2xl font-bold">
            {loading ? '...' : `${balance} ${account?.asset || ''}`}
          </p>
        </div>
        <div className="border rounded-lg p-4">
          <p className="text-sm text-muted-foreground">Total Calls</p>
          <p className="text-2xl font-bold">
            {loading ? '...' : stats?.total_calls?.toLocaleString() ?? '0'}
          </p>
        </div>
        <div className="border rounded-lg p-4">
          <p className="text-sm text-muted-foreground">Total Tokens</p>
          <p className="text-2xl font-bold">
            {loading ? '...' : stats?.total_tokens?.toLocaleString() ?? '0'}
          </p>
        </div>
      </div>

      {stats && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="border rounded-lg p-4">
            <p className="text-sm text-muted-foreground">Total Credits</p>
            <p className="text-2xl font-bold">{stats.total_credits.toFixed(2)}</p>
          </div>
          <div className="border rounded-lg p-4">
            <p className="text-sm text-muted-foreground">Avg Latency</p>
            <p className="text-2xl font-bold">{stats.avg_latency_ms.toFixed(0)} ms</p>
          </div>
        </div>
      )}

      <div className="border rounded-lg p-4">
        <h2 className="font-semibold mb-2">Account Info</h2>
        <p className="text-sm text-muted-foreground">Username: {user?.username}</p>
        <p className="text-sm text-muted-foreground">Email: {user?.email}</p>
        <p className="text-sm text-muted-foreground">User ID: {user?.id}</p>
      </div>
    </div>
  )
}
