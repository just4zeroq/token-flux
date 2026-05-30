import { createFileRoute, Navigate } from '@tanstack/react-router'
import { useEffect, useState, useRef } from 'react'
import { useAuthStore } from '../stores/auth'
import { apiGet, apiPost, apiDel } from '../api/client'
import { Sidebar } from '../components/Sidebar'
import { StatCard } from '../components/StatCard'

export const Route = createFileRoute('/console')({
  component: ConsolePage,
})

function formatUnits(v: number): string {
  const n = Math.abs(v)
  if (n >= 1_000_000_000) return (v / 1_000_000_000).toFixed(1) + 'B'
  if (n >= 1_000_000) return (v / 1_000_000).toFixed(1) + 'M'
  if (n >= 10_000) return (v / 1_000).toFixed(1) + 'K'
  if (n === 0) return '0'
  return v.toFixed(v < 1 ? 4 : 0)
}

/* ───── Shared Types ───── */

interface AccountInfo { owner_type: string; owner_id: number; asset: string; balance_micro: number }
interface UsageStats { total_calls: number; total_tokens: number; total_credits: number; avg_latency_ms: number }
interface ApiKey { id: number; name: string; key: string; status: number; created_at: string }
interface UsageRecord { id: number; model: string; tokens: number; credits: number; latency_ms: number; created_at: string }
interface OrderInfo { order_no: string; status: string; amount_credits: number; created_at: string }
interface Transaction { id: number; type: string; entries: { delta: number }[]; created_at: string }
interface TxResponse { list: Transaction[]; total: number; page: number; page_size: number }
interface ProviderStats { total_revenue_credits: number; total_commission_credits: number; total_settlements: number; pending_revenue_credits: number; pending_count: number }

interface ModelUsageRank { model_code: string; model_name: string; call_count: number; total_tokens: number; total_credits: number }

/* ───── Main Console ───── */

function ConsolePage() {
  const token = useAuthStore((s) => s.token)
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)
  const [menuOpen, setMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const onDocClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) setMenuOpen(false)
    }
    document.addEventListener('mousedown', onDocClick)
    return () => document.removeEventListener('mousedown', onDocClick)
  }, [])

  if (!token) return <Navigate to="/login" />

  const params = new URLSearchParams(window.location.search)
  const [activeTab, setActiveTab] = useState(params.get('tab') || 'overview')

  const handleTabChange = (tab: string) => {
    setActiveTab(tab)
    window.history.replaceState(null, '', '/console?tab=' + tab)
  }

  const avatarLetter = user?.username?.charAt(0).toUpperCase() || 'U'

  return (
    <div className="flex h-screen overflow-hidden bg-background">
      <Sidebar activeTab={activeTab} onTabChange={handleTabChange} />
      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Top bar */}
        <header className="flex h-12 shrink-0 items-center justify-between border-b border-border bg-white px-5">
          <a href="/" className="flex items-center gap-1.5 text-xs text-muted-foreground transition-colors hover:text-foreground">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="m15 18-6-6 6-6"/></svg>
            Back to Home
          </a>
          <div ref={menuRef} className="relative">
            <button
              onClick={() => setMenuOpen(!menuOpen)}
              className="flex h-7 w-7 items-center justify-center rounded-full bg-primary text-[10px] font-bold text-primary-foreground transition-opacity hover:opacity-80"
            >
              {avatarLetter}
            </button>
            {menuOpen && (
              <div className="absolute right-0 top-full mt-2 w-44 overflow-hidden rounded-lg border border-border bg-white shadow-lg animate-scale-in">
                <div className="border-b border-border px-3 py-2">
                  <p className="text-sm font-medium text-foreground">{user?.username}</p>
                  <p className="text-xs text-muted-foreground truncate">{user?.email}</p>
                </div>
                <a
                  href="/console?tab=settings"
                  onClick={() => setMenuOpen(false)}
                  className="flex items-center gap-2 px-3 py-2 text-sm text-foreground transition-colors hover:bg-muted"
                >
                  Profile
                </a>
                <button
                  onClick={logout}
                  className="flex w-full items-center gap-2 px-3 py-2 text-sm text-destructive transition-colors hover:bg-muted"
                >
                  Logout
                </button>
              </div>
            )}
          </div>
        </header>
        <main className="flex-1 overflow-y-auto">
          <div className="mx-auto max-w-6xl p-6">
            {activeTab === 'overview' && <OverviewTab />}
            {activeTab === 'keys' && <KeysTab />}
            {activeTab === 'usage' && <UsageTab />}
            {activeTab === 'recharge' && <RechargeTab />}
            {activeTab === 'provider' && <ProviderTab onTabChange={handleTabChange} />}
            {activeTab === 'channels' && <ChannelsTab />}
            {activeTab === 'provider-models' && <ProviderModelsTab />}
            {activeTab === 'provider-keys' && <ProviderKeysTab />}
            {activeTab === 'settings' && <SettingsTab />}
          </div>
        </main>
      </div>
    </div>
  )
}

/* ───── Overview Tab ───── */

function OverviewTab() {
  const token = useAuthStore((s) => s.token)
  const user = useAuthStore((s) => s.user)
  const isProvider = Number(user?.role) === 1
  const [credits, setCredits] = useState<number | null>(null)
  const [wallet, setWallet] = useState<number | null>(null)
  const [points, setPoints] = useState<number | null>(null)
  const [stats, setStats] = useState<UsageStats | null>(null)
  const [keysCount, setKeysCount] = useState(0)
  const [recent, setRecent] = useState<UsageRecord[]>([])

  // Provider stats
  const [modelCount, setModelCount] = useState(0)
  const [providerRevenue, setProviderRevenue] = useState<ProviderStats | null>(null)

  // Model usage ranking
  const [modelUsageRank, setModelUsageRank] = useState<ModelUsageRank[]>([])

  useEffect(() => {
    if (!token) return

    const fetchBalance = async (asset: string) => {
      try {
        const res = await apiGet<AccountInfo>(`/billing/balance?asset=${asset}`, token)
        return res?.balance_micro ?? 0
      } catch { return 0 }
    }

    Promise.all([
      fetchBalance('credits'),
      fetchBalance('balance'),
      fetchBalance('points'),
      apiGet<UsageStats>('/usage/stats', token).catch(() => null),
      apiGet<ApiKey[]>('/users/me/keys', token).catch(() => []),
      apiGet<UsageRecord[]>('/usage/records?page=1&page_size=5', token).catch(() => []),
      // Provider stats
      isProvider ? apiGet<any>('/provider/llm/models', token).catch(() => ({ list: [] })) : Promise.resolve({ list: [] }),
      isProvider ? apiGet<ProviderStats>('/billing/provider-stats', token).catch(() => null) : Promise.resolve(null),
      // Model usage ranking
      apiGet<{ list: ModelUsageRank[] }>('/usage/stats-by-model', token).catch(() => ({ list: [] })),
    ]).then(([c, w, p, st, keys, rec, modelsRes, rev, ranking]) => {
      setCredits(c)
      setWallet(w)
      setPoints(p)
      setStats(st)
      setKeysCount(Array.isArray(keys) ? keys.length : 0)
      setRecent(rec)
      setModelCount(modelsRes?.list?.length ?? 0)
      setProviderRevenue(rev)
      setModelUsageRank(ranking?.list ?? [])
    })
  }, [token])

  const fmtValue = (v: number | null) => v != null ? formatUnits(v) : '—'

  return (
    <div className="space-y-6 animate-fade-in">
      <div>
        <h1 className="text-xl font-bold tracking-tight">Overview</h1>
        <p className="text-sm text-muted-foreground">Welcome back, {user?.username || 'Developer'}</p>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label="Credits" value={fmtValue(credits)} color="primary">
          <a href="/console?tab=recharge" className="mt-2 inline-flex h-7 items-center rounded-md bg-primary px-2.5 text-[11px] font-medium text-primary-foreground transition-opacity hover:opacity-90">
            + Recharge
          </a>
        </StatCard>
        <StatCard label="Wallet Balance" value={fmtValue(wallet)} color="default" />
        <StatCard label="Points" value={fmtValue(points)} color="accent" />
        <StatCard label="Active API Keys" value={keysCount} color="default" />
      </div>

      {/* Provider Revenue — shown before usage */}
      {isProvider && providerRevenue && (
        <div className="rounded-xl border border-primary/30 bg-primary/5 p-4">
          <p className="text-xs font-semibold text-primary mb-2 tracking-wider uppercase">Revenue</p>
          <div className="grid gap-3 sm:grid-cols-4">
            <div>
              <p className="text-xs text-muted-foreground">Total Revenue</p>
              <p className="text-lg font-bold tabular-nums">{formatUnits(providerRevenue.total_revenue_credits)}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Commission</p>
              <p className="text-lg font-bold tabular-nums">{formatUnits(providerRevenue.total_commission_credits)}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Settlements</p>
              <p className="text-lg font-bold tabular-nums">{providerRevenue.total_settlements}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Models</p>
              <p className="text-lg font-bold tabular-nums">{modelCount}</p>
            </div>
          </div>
          {/* Pending row */}
          <div className="mt-3 border-t border-primary/20 pt-3 grid gap-3 sm:grid-cols-2">
            <div>
              <p className="text-xs text-muted-foreground">Pending Revenue</p>
              <p className="text-base font-semibold tabular-nums text-yellow-500">
                {formatUnits(providerRevenue.pending_revenue_credits)}
              </p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Pending Settlements</p>
              <p className="text-base font-semibold tabular-nums">{providerRevenue.pending_count}</p>
            </div>
          </div>
        </div>
      )}

      {/* Recent Usage */}
      <div className="rounded-xl border border-border bg-card">
        <div className="flex items-center justify-between border-b border-border px-5 py-3">
          <h2 className="text-sm font-semibold">Recent Usage</h2>
          <a href="/console?tab=usage" className="text-xs text-muted-foreground hover:text-foreground transition-colors">View All →</a>
        </div>
        {recent.length === 0 ? (
          <p className="px-5 py-8 text-center text-xs text-muted-foreground">No usage yet. Start by creating an API key and making a request.</p>
        ) : (
          <div className="divide-y divide-border text-sm">
            {recent.map((r) => (
              <div key={r.id} className="flex items-center justify-between px-5 py-2.5">
                <div className="flex items-center gap-3">
                  <span className="font-medium">{r.model}</span>
                  <span className="text-xs text-muted-foreground">{r.tokens.toLocaleString()} tokens</span>
                </div>
                <span className="text-xs text-muted-foreground">{new Date(r.created_at).toLocaleString()}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Model Usage Ranking */}
      {modelUsageRank.length > 0 && (
        <div className="rounded-xl border border-border bg-card">
          <div className="flex items-center justify-between border-b border-border px-5 py-3">
            <h2 className="text-sm font-semibold">Model Usage Ranking</h2>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-muted/50">
                <tr>
                  <th className="p-3 text-left font-medium">Model</th>
                  <th className="p-3 text-right font-medium">Calls</th>
                  <th className="p-3 text-right font-medium">Tokens</th>
                  <th className="p-3 text-right font-medium">Credits</th>
                </tr>
              </thead>
              <tbody>
                {modelUsageRank.slice(0, 10).map((m, i) => (
                  <tr key={m.model_code} className="border-t border-border">
                    <td className="p-3 flex items-center gap-2">
                      <span className="text-xs text-muted-foreground w-5">{i + 1}.</span>
                      <span className="font-medium">{m.model_name || m.model_code}</span>
                    </td>
                    <td className="p-3 text-right tabular-nums">{m.call_count.toLocaleString()}</td>
                    <td className="p-3 text-right tabular-nums">{m.total_tokens.toLocaleString()}</td>
                    <td className="p-3 text-right tabular-nums">{formatUnits(m.total_credits)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}

/* ───── API Keys Tab ───── */

function KeysTab() {
  const token = useAuthStore((s) => s.token)
  const [keys, setKeys] = useState<ApiKey[]>([])
  const [loading, setLoading] = useState(true)
  const [showNew, setShowNew] = useState(false)
  const [newName, setNewName] = useState('')
  const [newKey, setNewKey] = useState('')

  const fetchKeys = async () => {
    if (!token) return
    try { setKeys(await apiGet<ApiKey[]>('/users/me/keys', token) || []) } catch { }
    finally { setLoading(false) }
  }
  useEffect(() => { fetchKeys() }, [token])

  const createKey = async () => {
    if (!token) return
    try {
      const res = await apiPost<{ id: number; key: string; name: string; status: number; created_at: string }>('/users/me/keys', { name: newName || 'Default' }, token)
      setNewKey(res.key); setShowNew(false); setNewName(''); fetchKeys()
    } catch {}
  }

  const deleteKey = async (id: number) => {
    if (!token) return
    try { await apiDel(`/users/me/keys/${id}`, token); fetchKeys() } catch {}
  }

  return (
    <div className="animate-fade-in">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-xl font-bold tracking-tight">API Keys</h1>
          <p className="text-sm text-muted-foreground">Manage your API access keys.</p>
        </div>
        <button onClick={() => setShowNew(!showNew)} className="h-8 rounded-lg bg-primary px-3 text-xs font-medium text-primary-foreground transition-opacity hover:opacity-90">
          {showNew ? 'Cancel' : '+ Create Key'}
        </button>
      </div>

      {newKey && (
        <div className="mb-4 rounded-xl border border-primary/30 bg-primary/5 p-4">
          <p className="text-sm font-medium text-primary">Key created — copy it now, it won't be shown again.</p>
          <code className="mt-2 block rounded-lg bg-background p-3 text-xs font-mono break-all">{newKey}</code>
          <button onClick={() => setNewKey('')} className="mt-2 text-xs text-muted-foreground underline">Dismiss</button>
        </div>
      )}

      {showNew && (
        <div className="mb-4 rounded-xl border border-border bg-card p-4 flex gap-2">
          <input value={newName} onChange={(e) => setNewName(e.target.value)} placeholder="Key name" className="h-9 flex-1 rounded-lg border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" />
          <button onClick={createKey} className="h-9 rounded-lg bg-primary px-4 text-sm font-medium text-primary-foreground">Create</button>
        </div>
      )}

      {loading ? (
        <div className="space-y-2">{Array.from({ length: 3 }).map((_, i) => <div key={i} className="h-14 animate-pulse rounded-lg bg-muted" />)}</div>
      ) : keys.length === 0 ? (
        <div className="py-16 text-center text-muted-foreground"><p>No API keys yet.</p></div>
      ) : (
        <div className="rounded-xl border border-border overflow-hidden">
          {keys.map((k) => (
            <div key={k.id} className="flex items-center justify-between border-b border-border p-4 last:border-b-0">
              <div>
                <p className="font-medium text-sm">{k.name || 'Unnamed'}</p>
                <code className="text-xs text-muted-foreground">{k.key?.substring(0, 16)}...</code>
              </div>
              <div className="flex items-center gap-3">
                <span className={`rounded-full px-2 py-0.5 text-xs ${k.status === 1 ? 'bg-green-500/10 text-green-500' : 'bg-red-500/10 text-red-500'}`}>
                  {k.status === 1 ? 'Active' : 'Disabled'}
                </span>
                <button onClick={() => deleteKey(k.id)} className="text-xs text-muted-foreground hover:text-destructive">Delete</button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

/* ───── Usage Tab ───── */

function UsageTab() {
  const token = useAuthStore((s) => s.token)
  const [stats, setStats] = useState<UsageStats | null>(null)
  const [records, setRecords] = useState<UsageRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [page, setPage] = useState(1)
  const pageSize = 20

  useEffect(() => {
    if (!token) return
    setLoading(true)
    Promise.all([
      apiGet<UsageStats>('/usage/stats', token).catch(() => null),
      apiGet<UsageRecord[]>(`/usage/records?page=${page}&page_size=${pageSize}`, token).catch(() => []),
    ]).then(([s, r]) => { setStats(s); setRecords(r) }).finally(() => setLoading(false))
  }, [token, page])

  return (
    <div className="animate-fade-in">
      <div className="mb-6">
        <h1 className="text-xl font-bold tracking-tight">Usage</h1>
        <p className="text-sm text-muted-foreground">Track your API consumption.</p>
      </div>

      <div className="mb-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label="Total Calls" value={loading ? '...' : stats?.total_calls?.toLocaleString() ?? '0'} />
        <StatCard label="Total Tokens" value={loading ? '...' : stats?.total_tokens?.toLocaleString() ?? '0'} />
        <StatCard label="Total Credits" value={loading ? '...' : (stats?.total_credits ?? 0).toFixed(2)} color="primary" />
        <StatCard label="Avg Latency" value={loading ? '...' : `${(stats?.avg_latency_ms ?? 0).toFixed(0)} ms`} color="accent" />
      </div>

      {loading ? (
        <div className="h-40 animate-pulse rounded-xl bg-muted" />
      ) : records.length === 0 ? (
        <div className="py-16 text-center text-muted-foreground"><p>No usage records yet.</p></div>
      ) : (
        <div className="rounded-xl border border-border overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-muted/50"><tr>
              <th className="p-3 text-left font-medium">Model</th>
              <th className="p-3 text-right font-medium">Tokens</th>
              <th className="p-3 text-right font-medium">Credits</th>
              <th className="p-3 text-right font-medium">Latency</th>
              <th className="p-3 text-left font-medium">Time</th>
            </tr></thead>
            <tbody>
              {records.map((r) => (
                <tr key={r.id} className="border-t border-border">
                  <td className="p-3">{r.model}</td>
                  <td className="p-3 text-right tabular-nums">{r.tokens.toLocaleString()}</td>
                  <td className="p-3 text-right tabular-nums">{r.credits.toFixed(4)}</td>
                  <td className="p-3 text-right tabular-nums">{r.latency_ms}ms</td>
                  <td className="p-3 text-muted-foreground">{new Date(r.created_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {records.length > 0 && (
        <div className="mt-4 flex items-center justify-center gap-2">
          <button onClick={() => setPage((p) => Math.max(1, p - 1))} disabled={page === 1} className="h-8 rounded-md border border-border px-3 text-xs disabled:opacity-40">← Prev</button>
          <span className="text-xs text-muted-foreground">Page {page}</span>
          <button onClick={() => setPage((p) => p + 1)} disabled={records.length < pageSize} className="h-8 rounded-md border border-border px-3 text-xs disabled:opacity-40">Next →</button>
        </div>
      )}
    </div>
  )
}

/* ───── Orders Tab ───── */

function OrdersTab() {
  const token = useAuthStore((s) => s.token)
  const [orders, setOrders] = useState<OrderInfo[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!token) return
    apiGet<OrderInfo[]>('/market/orders', token).then(setOrders).catch(() => {}).finally(() => setLoading(false))
  }, [token])

  return (
    <div className="animate-fade-in">
      <div className="mb-6">
        <h1 className="text-xl font-bold tracking-tight">Orders</h1>
        <p className="text-sm text-muted-foreground">Your recharge order history.</p>
      </div>

      {loading ? (
        <div className="space-y-2">{Array.from({ length: 3 }).map((_, i) => <div key={i} className="h-14 animate-pulse rounded-lg bg-muted" />)}</div>
      ) : orders.length === 0 ? (
        <div className="py-16 text-center text-muted-foreground"><p>No orders yet.</p><a href="/console?tab=recharge" className="mt-2 inline-block text-sm text-primary hover:underline">Recharge now →</a></div>
      ) : (
        <div className="rounded-xl border border-border overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-muted/50"><tr>
              <th className="p-3 text-left font-medium">Order No.</th>
              <th className="p-3 text-left font-medium">Status</th>
              <th className="p-3 text-right font-medium">Credits</th>
              <th className="p-3 text-left font-medium">Time</th>
            </tr></thead>
            <tbody>
              {orders.map((o) => (
                <tr key={o.order_no} className="border-t border-border">
                  <td className="p-3 font-mono text-xs">{o.order_no}</td>
                  <td className="p-3">
                    <span className={`rounded-full px-2 py-0.5 text-xs capitalize ${
                      o.status === 'completed' ? 'bg-green-500/10 text-green-500' :
                      o.status === 'pending' ? 'bg-yellow-500/10 text-yellow-500' :
                      'bg-muted text-muted-foreground'
                    }`}>{o.status}</span>
                  </td>
                  <td className="p-3 text-right tabular-nums">{o.amount_credits.toLocaleString()}</td>
                  <td className="p-3 text-muted-foreground">{new Date(o.created_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

/* ───── Transactions Tab ───── */

function TransactionsTab() {
  const token = useAuthStore((s) => s.token)
  const [data, setData] = useState<TxResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [page, setPage] = useState(1)
  const pageSize = 20

  useEffect(() => {
    if (!token) return
    setLoading(true)
    apiGet<TxResponse>(`/billing/transactions?page=${page}&page_size=${pageSize}`, token)
      .then(setData).catch(() => {}).finally(() => setLoading(false))
  }, [token, page])

  const totalPages = data ? Math.ceil(data.total / data.page_size) : 1

  return (
    <div className="animate-fade-in">
      <div className="mb-6">
        <h1 className="text-xl font-bold tracking-tight">Transactions</h1>
        <p className="text-sm text-muted-foreground">Your account transaction history.</p>
      </div>

      {loading ? (
        <div className="space-y-2">{Array.from({ length: 3 }).map((_, i) => <div key={i} className="h-14 animate-pulse rounded-lg bg-muted" />)}</div>
      ) : !data || data.list.length === 0 ? (
        <div className="py-16 text-center text-muted-foreground"><p>No transactions yet.</p></div>
      ) : (
        <>
          <div className="rounded-xl border border-border overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-muted/50"><tr>
                <th className="p-3 text-left font-medium">ID</th>
                <th className="p-3 text-left font-medium">Type</th>
                <th className="p-3 text-right font-medium">Amount</th>
                <th className="p-3 text-left font-medium">Time</th>
              </tr></thead>
              <tbody>
                {data.list.map((t) => {
                  const totalDelta = t.entries?.reduce((sum, e) => sum + e.delta, 0) ?? 0
                  return (
                    <tr key={t.id} className="border-t border-border">
                      <td className="p-3 font-mono text-xs">{t.id}</td>
                      <td className="p-3 capitalize">{t.type.replace(/_/g, ' ')}</td>
                      <td className={`p-3 text-right tabular-nums ${totalDelta >= 0 ? 'text-green-500' : 'text-red-500'}`}>
                        {totalDelta >= 0 ? '+' : ''}{(totalDelta / 1_000_000).toFixed(6)}
                      </td>
                      <td className="p-3 text-muted-foreground">{new Date(t.created_at).toLocaleString()}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
          <div className="mt-4 flex items-center justify-center gap-2">
            <button onClick={() => setPage((p) => Math.max(1, p - 1))} disabled={page === 1} className="h-8 rounded-md border border-border px-3 text-xs disabled:opacity-40">← Prev</button>
            <span className="text-xs text-muted-foreground">Page {page} of {totalPages}</span>
            <button onClick={() => setPage((p) => p + 1)} disabled={page >= totalPages} className="h-8 rounded-md border border-border px-3 text-xs disabled:opacity-40">Next →</button>
          </div>
        </>
      )}
    </div>
  )
}

/* ───── Settings Tab ───── */

function SettingsTab() {
  const user = useAuthStore((s) => s.user)
  return (
    <div className="animate-fade-in">
      <div className="mb-6">
        <h1 className="text-xl font-bold tracking-tight">Settings</h1>
        <p className="text-sm text-muted-foreground">Manage your account settings.</p>
      </div>

      <div className="space-y-4 max-w-lg">
        <div className="rounded-xl border border-border bg-card p-5">
          <p className="text-sm font-medium">Profile</p>
          <div className="mt-3 space-y-2 text-sm text-muted-foreground">
            <div className="flex justify-between"><span>Username</span><span className="text-foreground">{user?.username || '—'}</span></div>
            <div className="flex justify-between"><span>Email</span><span className="text-foreground">{user?.email || '—'}</span></div>
            <div className="flex justify-between"><span>User ID</span><span className="text-foreground font-mono">{user?.id || '—'}</span></div>
          </div>
        </div>
      </div>
    </div>
  )
}

/* ───── Recharge Tab ───── */

function RechargeTab() {
  const [amount, setAmount] = useState(100)
  const [payUrl, setPayUrl] = useState('')
  const [loading, setLoading] = useState(false)

  const createRecharge = async (channel: string) => {
    setLoading(true)
    try {
      const res = await fetch('/api/v1/payment/recharge', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ channel, amount_credits: amount }),
      })
      const data = await res.json()
      if (data.pay_url) window.open(data.pay_url, '_blank')
      else if (data.qrcode) setPayUrl(data.qrcode)
    } catch {} finally { setLoading(false) }
  }

  const presets = [100, 500, 1000, 5000]

  return (
    <div className="animate-fade-in max-w-lg">
      <div className="mb-6">
        <h1 className="text-xl font-bold tracking-tight">Recharge</h1>
        <p className="text-sm text-muted-foreground">Add credits to your account. 1 CNY = 10 credits.</p>
      </div>

      <div className="rounded-xl border border-border bg-card p-6">
        <p className="text-sm font-medium mb-3">Amount (Credits)</p>
        <div className="flex flex-wrap gap-2 mb-4">
          {presets.map((p) => (
            <button key={p} onClick={() => setAmount(p)}
              className={`h-9 rounded-lg border px-4 text-sm transition-colors ${amount === p ? 'border-primary bg-primary/10 text-primary' : 'border-border hover:border-primary/50'}`}>
              {p.toLocaleString()}
            </button>
          ))}
        </div>
        <input type="number" value={amount} onChange={(e) => setAmount(Number(e.target.value))} min={1}
          className="mb-6 h-10 w-full rounded-lg border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" />

        <div className="flex gap-3">
          <button onClick={() => createRecharge('alipay')} disabled={loading || amount < 1}
            className="flex-1 h-10 rounded-lg bg-primary text-sm font-medium text-primary-foreground transition-opacity hover:opacity-90 disabled:opacity-40">
            {loading ? 'Processing...' : 'Alipay'}
          </button>
          <button onClick={() => createRecharge('wechat')} disabled={loading || amount < 1}
            className="flex-1 h-10 rounded-lg bg-accent text-sm font-medium text-accent-foreground transition-opacity hover:opacity-90 disabled:opacity-40">
            {loading ? 'Processing...' : 'WeChat Pay'}
          </button>
        </div>

        {payUrl && (
          <div className="mt-4 rounded-lg bg-muted p-4 text-center">
            <p className="text-xs text-muted-foreground">Scan QR code to pay</p>
            <img src={payUrl} alt="Payment QR" className="mx-auto mt-2 h-40 w-40" />
          </div>
        )}
      </div>
    </div>
  )
}

/* ───── Provider Tab ───── */

function ProviderTab({ onTabChange }: { onTabChange: (t: string) => void }) {
  const token = useAuthStore((s) => s.token)
  const user = useAuthStore((s) => s.user)
  const isProvider = Number(user?.role) === 1

  if (!isProvider) {
    return (
      <div className="animate-fade-in max-w-lg mx-auto py-16 text-center">
        <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-primary/10 text-3xl text-primary">⊕</div>
        <h1 className="mt-6 text-2xl font-bold tracking-tight">Become a Provider</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Publish your models on AI Platform and reach thousands of developers.
          Simple integration with automated settlement.
        </p>
        <div className="mt-8 flex flex-col gap-3">
          <a href="/providers-apply" className="inline-flex h-10 items-center justify-center rounded-lg bg-primary px-6 text-sm font-medium text-primary-foreground transition-opacity hover:opacity-90">
            Apply Now
          </a>
        </div>
      </div>
    )
  }

  const [models, setModels] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!token) return
    Promise.all([
      apiGet<any>('/provider/llm/models', token).catch(() => ({ list: [] })),
    ]).then(([m]) => {
      setModels(m?.list ?? [])
    }).finally(() => setLoading(false))
  }, [token])

  return (
    <div className="animate-fade-in space-y-6">
      <div>
        <h1 className="text-xl font-bold tracking-tight">Provider Dashboard</h1>
        <p className="text-sm text-muted-foreground">Manage your models and channels.</p>
      </div>

      <div className="grid gap-3 sm:grid-cols-3">
        <StatCard label="Models" value={loading ? '...' : models.length} color="primary" />
        <StatCard label="Channels" value="—" color="default" />
        <StatCard label="Active Keys" value="—" color="default" />
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        {[
          { key: 'channels', label: 'Channels', desc: 'Configure API protocol settings', icon: '⇆' },
          { key: 'provider-models', label: 'Models', desc: 'Manage your published models', icon: '▤' },
          { key: 'provider-keys', label: 'Model Keys', desc: 'Upstream API key management', icon: '⚷' },
        ].map((item) => (
          <button key={item.key} onClick={() => onTabChange(item.key)}
            className="group text-left rounded-xl border border-border bg-card p-5 transition-all hover:border-primary/50 hover:bg-card/80"
          >
            <span className="text-2xl">{item.icon}</span>
            <h3 className="mt-3 font-semibold">{item.label}</h3>
            <p className="mt-1 text-sm text-muted-foreground">{item.desc}</p>
          </button>
        ))}
      </div>
    </div>
  )
}

/* ───── Channels Tab ───── */

function ChannelsTab() {
  const token = useAuthStore((s) => s.token)
  const [channels, setChannels] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({ code: '', name: '', description: '' })
  const [protocols, setProtocols] = useState<{protocol: string, base_url: string}[]>([])
  const [selProto, setSelProto] = useState('')
  const [selURL, setSelURL] = useState('')
  const [saving, setSaving] = useState(false)
  const [filterStatus, setFilterStatus] = useState('')
  const [filterType, setFilterType] = useState('')

  const PROTOCOLS = [
    { value: 'openai-compatible', label: 'OpenAI' },
    { value: 'anthropic-compatible', label: 'Anthropic' },
  ]

  const addProtocol = () => {
    if (!selProto || !selURL.trim()) return
    if (protocols.find(p => p.protocol === selProto)) return
    setProtocols([...protocols, { protocol: selProto, base_url: selURL.trim() }])
    setSelProto(''); setSelURL('')
  }

  const removeProtocol = (proto: string) => {
    setProtocols(protocols.filter(p => p.protocol !== proto))
  }

  const fetch = () => {
    if (!token) return
    setLoading(true)
    const params = new URLSearchParams()
    if (filterStatus) params.set('status', filterStatus)
    if (filterType) params.set('source_type', filterType)
    apiGet<any>('/provider/llm/channels?' + params.toString(), token)
      .then((d: any) => setChannels(d?.list ?? []))
      .catch(() => {})
      .finally(() => setLoading(false))
  }
  useEffect(() => { fetch() }, [token, filterStatus, filterType])

  const create = async () => {
    if (!token) return
    setSaving(true)
    try {
      await apiPost<any>('/provider/llm/channels', { ...form, protocols }, token)
      setShowForm(false)
      setForm({ code: '', name: '', description: '' })
      setProtocols([])
      fetch()
    } catch {} finally { setSaving(false) }
  }

  return (
    <div className="animate-fade-in">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold tracking-tight">Channels</h1>
          <p className="text-sm text-muted-foreground">Configure your LLM API channels.</p>
        </div>
        <button onClick={() => setShowForm(!showForm)} className="h-8 rounded-lg bg-primary px-3 text-xs font-medium text-primary-foreground hover:opacity-90">
          {showForm ? 'Cancel' : '+ New Channel'}
        </button>
      </div>

      {/* Create Form */}
      {showForm && (
        <div className="mb-6 rounded-xl border border-border bg-card p-5 space-y-3">
          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <label className="block text-xs font-medium mb-1">Code *</label>
              <input value={form.code} onChange={(e) => setForm({ ...form, code: e.target.value })}
                className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" placeholder="my-channel" />
            </div>
            <div>
              <label className="block text-xs font-medium mb-1">Name *</label>
              <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })}
                className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" placeholder="My Channel" />
            </div>
          </div>
          <div>
            <label className="block text-xs font-medium mb-1">Description</label>
            <input value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })}
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" placeholder="Channel description" />
          </div>

          {/* Protocol entries */}
          <div>
            <label className="block text-xs font-medium mb-1">Protocols *</label>
            <div className="flex gap-2 mb-2">
              <select value={selProto} onChange={(e) => setSelProto(e.target.value)}
                className="h-9 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring w-44">
                <option value="">Select protocol...</option>
                {PROTOCOLS.map(p => (
                  <option key={p.value} value={p.value} disabled={protocols.some(x => x.protocol === p.value)}>{p.label}</option>
                ))}
              </select>
              <input value={selURL} onChange={(e) => setSelURL(e.target.value)}
                className="h-9 flex-1 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" placeholder="https://api.openai.com/v1" />
              <button onClick={addProtocol} disabled={!selProto || !selURL.trim()}
                className="h-9 rounded-md bg-primary px-3 text-xs font-medium text-primary-foreground disabled:opacity-40">+ Add</button>
            </div>
            {protocols.length === 0 ? (
              <p className="text-xs text-muted-foreground">No protocols added yet.</p>
            ) : (
              <div className="space-y-1">
                {protocols.map(p => (
                  <div key={p.protocol} className="flex items-center gap-2 rounded-md border border-border bg-muted/30 px-3 py-2 text-sm">
                    <span className="font-medium">{PROTOCOLS.find(x => x.value === p.protocol)?.label || p.protocol}</span>
                    <span className="text-muted-foreground">→</span>
                    <code className="flex-1 text-xs">{p.base_url}</code>
                    <button onClick={() => removeProtocol(p.protocol)} className="text-xs text-destructive hover:underline">Remove</button>
                  </div>
                ))}
              </div>
            )}
          </div>

          <button onClick={create} disabled={saving || !form.code || !form.name || protocols.length === 0}
            className="h-8 rounded-lg bg-primary px-4 text-xs font-medium text-primary-foreground hover:opacity-90 disabled:opacity-40">
            {saving ? 'Creating...' : 'Create Channel'}
          </button>
        </div>
      )}

      {/* Filter bar */}
      <div className="mb-4 flex gap-2">
        <select value={filterStatus} onChange={(e) => setFilterStatus(e.target.value)}
          className="h-8 rounded-md border border-input bg-background px-2 text-xs focus:outline-none focus:ring-2 focus:ring-ring">
          <option value="">All Status</option>
          <option value="active">Active</option>
          <option value="pending">Pending</option>
          <option value="disabled">Disabled</option>
        </select>
        <select value={filterType} onChange={(e) => setFilterType(e.target.value)}
          className="h-8 rounded-md border border-input bg-background px-2 text-xs focus:outline-none focus:ring-2 focus:ring-ring">
          <option value="">All Types</option>
          <option value="admin">Built-in</option>
          <option value="provider">Provider</option>
        </select>
      </div>

      {/* List */}
      {loading ? (
        <div className="h-32 animate-pulse rounded-xl bg-muted" />
      ) : channels.length === 0 ? (
        <div className="py-16 text-center text-muted-foreground">
          <p className="text-sm">No channels found.</p>
          <p className="mt-1 text-xs">Try adjusting filters or create a new channel.</p>
        </div>
      ) : (
        <div className="rounded-xl border border-border overflow-hidden">
          {channels.map((ch: any) => (
            <div key={ch.id} className="flex items-center justify-between border-b border-border p-4 last:border-b-0">
              <div>
                <div className="flex items-center gap-2">
                  <p className="font-medium text-sm">{ch.name || ch.code}</p>
                  <span className={`rounded-full px-1.5 py-0.5 text-[10px] ${
                    ch.source_type === 'admin' ? 'bg-blue-500/10 text-blue-500' : 'bg-violet-500/10 text-violet-500'
                  }`}>{ch.source_type === 'admin' ? 'Built-in' : 'Provider'}</span>
                </div>
                <p className="text-xs text-muted-foreground mt-0.5">{ch.description}</p>
                {ch.protocols_json && (
                  <div className="mt-1 flex flex-wrap gap-1">
                    {(() => {
                      try {
                        return Object.keys(JSON.parse(ch.protocols_json)).map((k: string) => (
                          <span key={k} className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">{k}</span>
                        ))
                      } catch { return null }
                    })()}
                  </div>
                )}
              </div>
              <span className={`rounded-full px-2 py-0.5 text-xs capitalize ${
                ch.status === 'active' ? 'bg-green-500/10 text-green-500' :
                ch.status === 'pending' ? 'bg-yellow-500/10 text-yellow-500' :
                'bg-muted text-muted-foreground'
              }`}>{ch.status}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

/* ───── Provider Models Tab ───── */

function ProviderModelsTab() {
  const token = useAuthStore((s) => s.token)
  const [models, setModels] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({ developer_name: '', model_name: '', model_code: '', display_name: '', model_family: '', description: '', capabilities_json: '["chat","completion"]', context_window: 4096 })
  const [saving, setSaving] = useState(false)
  const [filterStatus, setFilterStatus] = useState('')
  const [filterType, setFilterType] = useState('')

  const fetch = () => {
    if (!token) return
    setLoading(true)
    const params = new URLSearchParams()
    if (filterStatus) params.set('status', filterStatus)
    if (filterType) params.set('source_type', filterType)
    apiGet<any>('/provider/llm/models?' + params.toString(), token)
      .then((d: any) => setModels(d?.list ?? []))
      .catch(() => {})
      .finally(() => setLoading(false))
  }
  useEffect(() => { fetch() }, [token, filterStatus, filterType])

  const create = async () => {
    if (!token) return
    setSaving(true)
    try {
      await apiPost<any>('/provider/llm/models', form, token)
      setShowForm(false)
      setForm({ developer_name: '', model_name: '', model_code: '', display_name: '', model_family: '', description: '', capabilities_json: '["chat","completion"]', context_window: 4096 })
      fetch()
    } catch {} finally { setSaving(false) }
  }

  return (
    <div className="animate-fade-in">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold tracking-tight">My Models</h1>
          <p className="text-sm text-muted-foreground">Models you have published on the platform.</p>
        </div>
        <button onClick={() => setShowForm(!showForm)} className="h-8 rounded-lg bg-primary px-3 text-xs font-medium text-primary-foreground hover:opacity-90">
          {showForm ? 'Cancel' : '+ New Model'}
        </button>
      </div>

      {/* Create Form */}
      {showForm && (
        <div className="mb-6 rounded-xl border border-border bg-card p-5 space-y-3">
          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <label className="block text-xs font-medium mb-1">Developer Name *</label>
              <input value={form.developer_name} onChange={(e) => setForm({ ...form, developer_name: e.target.value })}
                className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" />
            </div>
            <div>
              <label className="block text-xs font-medium mb-1">Model Name *</label>
              <input value={form.model_name} onChange={(e) => setForm({ ...form, model_name: e.target.value })}
                className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" />
            </div>
            <div>
              <label className="block text-xs font-medium mb-1">Model Code</label>
              <input value={form.model_code} onChange={(e) => setForm({ ...form, model_code: e.target.value })}
                className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" />
            </div>
            <div>
              <label className="block text-xs font-medium mb-1">Display Name</label>
              <input value={form.display_name} onChange={(e) => setForm({ ...form, display_name: e.target.value })}
                className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" />
            </div>
            <div>
              <label className="block text-xs font-medium mb-1">Family</label>
              <input value={form.model_family} onChange={(e) => setForm({ ...form, model_family: e.target.value })}
                className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" />
            </div>
            <div>
              <label className="block text-xs font-medium mb-1">Context Window</label>
              <input type="number" value={form.context_window} onChange={(e) => setForm({ ...form, context_window: Number(e.target.value) })}
                className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" />
            </div>
          </div>
          <div>
            <label className="block text-xs font-medium mb-1">Capabilities JSON *</label>
            <textarea value={form.capabilities_json} onChange={(e) => setForm({ ...form, capabilities_json: e.target.value })}
              rows={2} className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-ring" />
          </div>
          <button onClick={create} disabled={saving || !form.developer_name || !form.model_name}
            className="h-8 rounded-lg bg-primary px-4 text-xs font-medium text-primary-foreground hover:opacity-90 disabled:opacity-40">
            {saving ? 'Creating...' : 'Create Model'}
          </button>
        </div>
      )}

      {/* Filter bar */}
      <div className="mb-4 flex gap-2">
        <select value={filterStatus} onChange={(e) => setFilterStatus(e.target.value)}
          className="h-8 rounded-md border border-input bg-background px-2 text-xs focus:outline-none focus:ring-2 focus:ring-ring">
          <option value="">All Status</option>
          <option value="active">Active</option>
          <option value="pending">Pending</option>
          <option value="disabled">Disabled</option>
        </select>
        <select value={filterType} onChange={(e) => setFilterType(e.target.value)}
          className="h-8 rounded-md border border-input bg-background px-2 text-xs focus:outline-none focus:ring-2 focus:ring-ring">
          <option value="">All Types</option>
          <option value="admin">Built-in</option>
          <option value="provider">Provider</option>
        </select>
      </div>

      {/* List */}
      {loading ? (
        <div className="grid gap-3 md:grid-cols-3">
          {[1,2,3].map(i => <div key={i} className="h-28 animate-pulse rounded-xl bg-muted" />)}
        </div>
      ) : models.length === 0 ? (
        <div className="py-16 text-center text-muted-foreground">
          <p className="text-sm">No models found.</p>
          <p className="mt-1 text-xs">Try adjusting filters or create a new model.</p>
        </div>
      ) : (
        <div className="grid gap-3 md:grid-cols-3">
          {models.map((m: any) => (
            <div key={m.id} className="rounded-xl border border-border bg-card p-5">
              <div className="flex items-center gap-2">
                <p className="text-xs text-muted-foreground">{m.developer_name || 'Unknown'}</p>
                <span className={`rounded-full px-1.5 py-0.5 text-[10px] ${
                  m.source_type === 'admin' ? 'bg-blue-500/10 text-blue-500' : 'bg-violet-500/10 text-violet-500'
                }`}>{m.source_type === 'admin' ? 'Built-in' : 'Provider'}</span>
              </div>
              <p className="mt-1 font-semibold">{m.model_name || m.model_code}</p>
              <div className="mt-3 flex flex-wrap gap-1">
                {m.model_family && <span className="rounded-full bg-primary/10 px-2 py-0.5 text-xs text-primary">{m.model_family}</span>}
                {m.context_window > 0 && <span className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">{(m.context_window / 1000).toFixed(0)}K ctx</span>}
                <span className={`rounded-full px-2 py-0.5 text-xs capitalize ${
                  m.status === 'active' ? 'bg-green-500/10 text-green-500' :
                  m.status === 'pending' ? 'bg-yellow-500/10 text-yellow-500' :
                  'bg-muted text-muted-foreground'
                }`}>{m.status || 'pending'}</span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

/* ───── Provider Keys Tab ───── */

function ProviderKeysTab() {
  const token = useAuthStore((s) => s.token)
  const [keys, setKeys] = useState<any[]>([])
  const [channels, setChannels] = useState<any[]>([])
  const [models, setModels] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({ channel_id: 0, name: '', key: '', quota_limit_credits: 1000000 })
  const [saving, setSaving] = useState(false)
  // Inline model binding fields (one model per key creation)
  const [bf, setBf] = useState({ model_spec_id: 0,
    cache_hit: 0, cache_miss: 0, output: 0 })
  const [expanded, setExpanded] = useState<Record<number, boolean>>({})
  const [bindings, setBindings] = useState<Record<number, any[]>>({})
  const [loadingBindings, setLoadingBindings] = useState<Record<number, boolean>>({})
  const [prices, setPrices] = useState<Record<number, any[]>>({}) // model_spec_id → prices
  const [bindForm, setBindForm] = useState<Record<number, { model_spec_id: number;
    cache_hit: number; cache_miss: number; output: number }>>({})
  const [savingBind, setSavingBind] = useState<Record<number, boolean>>({})

  const fetchPricesForModel = async (modelSpecId: number) => {
    if (prices[modelSpecId]) return prices[modelSpecId]
    try {
      const res = await apiGet<any>('/provider/llm/models/' + modelSpecId + '/prices', token)
      const list = res?.list ?? []
      setPrices((prev) => ({ ...prev, [modelSpecId]: list }))
      return list
    } catch { return [] }
  }

  const toggleExpand = async (keyId: number) => {
    if (expanded[keyId]) {
      setExpanded({ ...expanded, [keyId]: false })
      return
    }
    setExpanded({ ...expanded, [keyId]: true })
    if (!bindings[keyId]) {
      setLoadingBindings({ ...loadingBindings, [keyId]: true })
      try {
        const res = await apiGet<any>('/provider/llm/model-keys/' + keyId + '/models', token)
        const list = res?.list ?? []
        setBindings({ ...bindings, [keyId]: list })
        // Also fetch prices for each unique model_spec_id in bindings
        const specIds = [...new Set(list.map((b: any) => b.model_spec_id))]
        for (const sid of specIds) {
          if (!prices[sid]) fetchPricesForModel(sid)
        }
      } catch {} finally {
        setLoadingBindings({ ...loadingBindings, [keyId]: false })
      }
    }
  }

  const fetch = () => {
    if (!token) return
    setLoading(true)
    Promise.all([
      apiGet<any>('/provider/llm/model-keys', token).catch(() => ({ list: [] })),
      apiGet<any>('/provider/llm/channels?status=active', token).catch(() => ({ list: [] })),
      apiGet<any>('/provider/llm/models', token).catch(() => ({ list: [] })),
    ]).then(([k, c, m]) => {
      setKeys(k?.list ?? [])
      setChannels(c?.list ?? [])
      setModels(m?.list ?? [])
    }).finally(() => setLoading(false))
  }
  useEffect(() => { fetch() }, [token])

  const create = async () => {
    if (!token) return
    setSaving(true)
    try {
      const body: any = { channel_id: form.channel_id, name: form.name, key: form.key, quota_limit_credits: form.quota_limit_credits }
      // Include model binding if model selected
      if (bf.model_spec_id) {
        const ms = models.find(function(m) { return m.id === bf.model_spec_id })
        body.model_bindings = [{
          model_spec_id: bf.model_spec_id,
          upstream_model_name: ms ? (ms.model_name || ms.model_code) : 'model-' + bf.model_spec_id,
          cache_hit_price_per_1k: bf.cache_hit,
          cache_miss_price_per_1k: bf.cache_miss,
          output_price_per_1k: bf.output,
        }]
      }
      await apiPost<any>('/provider/llm/model-keys', body, token)
      setShowForm(false)
      setForm({ channel_id: 0, name: '', key: '', quota_limit_credits: 1000000 })
      setBf({ model_spec_id: 0, cache_hit: 0, cache_miss: 0, output: 0 })
      fetch()
    } catch {} finally { setSaving(false) }
  }

  const BIND_INIT = { model_spec_id: 0, cache_hit: 0, cache_miss: 0, output: 0 }

  const bindModel = async (keyId: number) => {
    if (!token) return
    const bf = bindForm[keyId] || BIND_INIT
    if (!bf.model_spec_id) return
    setSavingBind({ ...savingBind, [keyId]: true })
    try {
      const ms = models.find(function(m) { return m.id === bf.model_spec_id })
      const upstreamName = ms ? (ms.model_name || ms.model_code) : 'model-' + bf.model_spec_id
      // Upsert prices for both chat & completion
      for (const cap of ['chat', 'completion']) {
        await apiPost<any>('/provider/llm/models/' + bf.model_spec_id + '/prices', {
          capability: cap, cache_hit_price_per_1k: bf.cache_hit, cache_miss_price_per_1k: bf.cache_miss, output_price_per_1k: bf.output
        }, token).catch(() => {})
      }
      // Bind key to model
      await apiPost<any>('/provider/llm/model-keys/' + keyId + '/models', {
        model_spec_id: bf.model_spec_id, upstream_model_name: upstreamName
      }, token)
      setBindForm({ ...bindForm, [keyId]: { ...BIND_INIT } })
      const res = await apiGet<any>('/provider/llm/model-keys/' + keyId + '/models', token)
      setBindings({ ...bindings, [keyId]: res?.list ?? [] })
    } catch {} finally { setSavingBind({ ...savingBind, [keyId]: false }) }
  }

  const triggerTest = async (kmId: number) => {
    if (!token) return
    try { await apiPost<any>('/provider/llm/key-models/' + kmId + '/test', {}, token) } catch {}
  }

  return (
    <div className="animate-fade-in">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold tracking-tight">Model Keys</h1>
          <p className="text-sm text-muted-foreground">Upstream API keys for your models.</p>
        </div>
        <button onClick={() => setShowForm(!showForm)} className="h-8 rounded-lg bg-primary px-3 text-xs font-medium text-primary-foreground hover:opacity-90">
          {showForm ? 'Cancel' : '+ New Key'}
        </button>
      </div>

      {showForm && (
        <div className="mb-6 rounded-xl border border-border bg-card p-5 space-y-3 max-w-lg">
          <div>
            <label className="block text-xs font-medium mb-1">Channel *</label>
            <select value={form.channel_id} onChange={(e) => setForm({ ...form, channel_id: Number(e.target.value) })}
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring">
              <option value={0}>Select channel...</option>
              {channels.map((ch: any) => <option key={ch.id} value={ch.id}>{ch.name || ch.code}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-xs font-medium mb-1">Name</label>
            <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })}
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" placeholder="My API Key" />
          </div>
          <div>
            <label className="block text-xs font-medium mb-1">Key *</label>
            <input value={form.key} onChange={(e) => setForm({ ...form, key: e.target.value })}
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-ring" placeholder="sk-..." />
          </div>
          <div>
            <label className="block text-xs font-medium mb-1">Quota Limit (credits)</label>
            <input type="number" value={form.quota_limit_credits} onChange={(e) => setForm({ ...form, quota_limit_credits: Number(e.target.value) })}
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" />
          </div>

          {/* ── Inline model binding + pricing ── */}
          <hr className="border-border" />
          <p className="text-xs font-semibold text-foreground">Model Binding (optional — add during creation)</p>

          <div className="grid grid-cols-1 gap-2">
            <div>
              <label className="block text-[10px] font-medium text-muted-foreground">Model</label>
              <select value={bf.model_spec_id} onChange={(e) => setBf({ ...bf, model_spec_id: Number(e.target.value) })}
                className="h-8 w-full rounded-md border border-input bg-background px-2 text-xs focus:outline-none focus:ring-2 focus:ring-ring">
                <option value={0}>Select...</option>
                {models.filter((m: any) => m.status === 'active').map((m: any) => (
                  <option key={m.id} value={m.id}>{m.model_name || m.model_code}</option>
                ))}
              </select>
            </div>
          </div>

          <p className="text-xs font-medium text-foreground pt-1">Pricing (credits per 1K tokens, 0 = free)</p>
          <div className="grid grid-cols-3 gap-2">
            <div>
              <label className="block text-[10px] font-medium text-muted-foreground">Input Cache Hit</label>
              <input type="number" value={bf.cache_hit} onChange={(e) => setBf({ ...bf, cache_hit: Number(e.target.value) })}
                className="h-8 w-full rounded-md border border-input bg-background px-2 text-xs focus:outline-none focus:ring-2 focus:ring-ring" placeholder="0" />
            </div>
            <div>
              <label className="block text-[10px] font-medium text-muted-foreground">Input Cache Miss</label>
              <input type="number" value={bf.cache_miss} onChange={(e) => setBf({ ...bf, cache_miss: Number(e.target.value) })}
                className="h-8 w-full rounded-md border border-input bg-background px-2 text-xs focus:outline-none focus:ring-2 focus:ring-ring" placeholder="0" />
            </div>
            <div>
              <label className="block text-[10px] font-medium text-muted-foreground">Output</label>
              <input type="number" value={bf.output} onChange={(e) => setBf({ ...bf, output: Number(e.target.value) })}
                className="h-8 w-full rounded-md border border-input bg-background px-2 text-xs focus:outline-none focus:ring-2 focus:ring-ring" placeholder="0" />
            </div>
          </div>

          <button onClick={create} disabled={saving || !form.channel_id || !form.key}
            className="h-8 rounded-lg bg-primary px-4 text-xs font-medium text-primary-foreground hover:opacity-90 disabled:opacity-40">
            {saving ? 'Creating...' : 'Create Key'}
          </button>
        </div>
      )}

      {loading ? (
        <div className="h-32 animate-pulse rounded-xl bg-muted" />
      ) : keys.length === 0 ? (
        <div className="py-16 text-center text-muted-foreground">
          <p className="text-sm">No model keys yet.</p>
        </div>
      ) : (
        <div className="rounded-xl border border-border overflow-hidden">
          {keys.map((k: any) => {
            const isExpanded = expanded[k.id]
            const bf = bindForm[k.id] || BIND_INIT
            return (
              <div key={k.id}>
                <div className="flex items-center justify-between border-b border-border p-4 hover:bg-muted/20 cursor-pointer"
                     onClick={() => toggleExpand(k.id)}>
                    <div className="flex items-center gap-3">
                      <span className={'text-xs transition-transform ' + (isExpanded ? 'rotate-90' : '')}>{'>'}</span>
                      <div>
                        <p className="font-medium text-sm">{k.name}</p>
                        <code className="text-xs text-muted-foreground">{k.key_masked}</code>
                        {k.quota_limit_credits > 0 && (
                          <div className="mt-1 min-w-[160px]">
                            <div className="flex items-center justify-between text-[10px] text-muted-foreground">
                              <span>Quota</span>
                              <span>{(k.quota_used_credits / 1_000_000).toLocaleString()} / {(k.quota_limit_credits / 1_000_000).toLocaleString()}</span>
                            </div>
                            <div className="mt-0.5 h-1.5 w-full rounded-full bg-muted overflow-hidden">
                              <div className={'h-full rounded-full transition-all ' + ((k.quota_used_credits / k.quota_limit_credits) > 0.8 ? 'bg-red-500' : 'bg-primary')}
                                   style={{ width: Math.min(100, (k.quota_used_credits / k.quota_limit_credits) * 100) + '%' }} />
                            </div>
                          </div>
                        )}
                      </div>
                    </div>
                  <div className="flex items-center gap-3">
                    <span className={'rounded-full px-2 py-0.5 text-xs ' + (k.status === 'active' ? 'bg-green-500/10 text-green-500' : 'bg-muted text-muted-foreground')}>
                      {k.status || 'pending'}
                    </span>
                  </div>
                </div>
                {isExpanded && (
                  <div className="border-b border-border bg-muted/20 p-4 space-y-3">
                    {loadingBindings[k.id] ? (
                      <p className="text-xs text-muted-foreground">Loading bindings...</p>
                    ) : (
                      <>
                        {(!bindings[k.id] || bindings[k.id].length === 0) ? (
                          <p className="text-xs text-muted-foreground">No models bound yet. Add one below.</p>
                        ) : (
                          <div className="space-y-2">
                            {bindings[k.id].map((km: any) => {
                              const ms = models.find((m: any) => m.id === km.model_spec_id)
                              return (
                                <div key={km.id} className="rounded-md border border-border bg-card px-3 py-2 text-sm">
                                  <div className="flex items-center justify-between">
                                    <div className="flex items-center gap-3">
                                      <span className="font-medium">{ms?.model_name || km.model_spec_id}</span>
                                      <code className="text-xs text-muted-foreground">→ {km.upstream_model_name}</code>
                                      <span className={'rounded-full px-1.5 py-0.5 text-[10px] ' + (km.status === 'active' ? 'bg-green-500/10 text-green-500' : 'bg-yellow-500/10 text-yellow-500')}>
                                        {km.status}
                                      </span>
                                      {km.consecutive_failures > 0 && (
                                        <span className="text-xs text-red-500">{km.consecutive_failures}x fail</span>
                                      )}
                                    </div>
                                    <button onClick={() => triggerTest(km.id)}
                                      className="rounded-md border border-border px-2 py-1 text-xs hover:bg-muted">Test</button>
                                  </div>
                                  {/* Prices — merged from first active price entry */}
                                  {(prices[km.model_spec_id] || []).filter((p: any) => p.status === 'active').length > 0 && (
                                    <div className="mt-1.5 text-[10px] text-muted-foreground">
                                      <span className="rounded bg-muted/50 px-1.5 py-0.5">
                                        hit={((prices[km.model_spec_id].find((p: any) => p.status === 'active') || {}).cache_hit_price_per_1k ?? 0).toLocaleString()} /
                                        miss={((prices[km.model_spec_id].find((p: any) => p.status === 'active') || {}).cache_miss_price_per_1k ?? 0).toLocaleString()} /
                                        out={((prices[km.model_spec_id].find((p: any) => p.status === 'active') || {}).output_price_per_1k ?? 0).toLocaleString()}
                                        &nbsp;credits/1K tokens
                                      </span>
                                    </div>
                                  )}
                                </div>
                              )
                            })}
                          </div>
                        )}
                        <div className="border-t border-border pt-3">
                          <p className="text-xs font-medium mb-2">Add Model</p>
                          <div className="flex flex-wrap gap-2 items-end">
                            <div>
                              <label className="block text-[10px] font-medium text-muted-foreground">Model</label>
                              <select value={bf.model_spec_id} onChange={(e) => setBindForm({ ...bindForm, [k.id]: { ...bf, model_spec_id: Number(e.target.value) } })}
                                className="h-8 rounded-md border border-input bg-background px-2 text-xs focus:outline-none focus:ring-2 focus:ring-ring w-36">
                                <option value={0}>Select...</option>
                                {models.filter((m: any) => m.status === 'active' && !(bindings[k.id] || []).find((b: any) => b.model_spec_id === m.id)).map((m: any) => (
                                  <option key={m.id} value={m.id}>{m.model_name || m.model_code}</option>
                                ))}
                              </select>
                            </div>
                          </div>
                          {/* Pricing */}
                          <div className="flex flex-wrap gap-2 items-end mt-2">
                            <div>
                              <label className="block text-[10px] font-medium text-muted-foreground">Input Cache Hit</label>
                              <input type="number" value={bf.cache_hit} onChange={(e) => setBindForm({ ...bindForm, [k.id]: { ...bf, cache_hit: Number(e.target.value) } })}
                                className="h-8 w-16 rounded-md border border-input bg-background px-2 text-xs focus:outline-none focus:ring-2 focus:ring-ring" />
                            </div>
                            <div>
                              <label className="block text-[10px] font-medium text-muted-foreground">Input Cache Miss</label>
                              <input type="number" value={bf.cache_miss} onChange={(e) => setBindForm({ ...bindForm, [k.id]: { ...bf, cache_miss: Number(e.target.value) } })}
                                className="h-8 w-16 rounded-md border border-input bg-background px-2 text-xs focus:outline-none focus:ring-2 focus:ring-ring" />
                            </div>
                            <div>
                              <label className="block text-[10px] font-medium text-muted-foreground">Output</label>
                              <input type="number" value={bf.output} onChange={(e) => setBindForm({ ...bindForm, [k.id]: { ...bf, output: Number(e.target.value) } })}
                                className="h-8 w-16 rounded-md border border-input bg-background px-2 text-xs focus:outline-none focus:ring-2 focus:ring-ring" />
                            </div>
                            <button onClick={() => bindModel(k.id)}
                              disabled={savingBind[k.id] || !bf.model_spec_id}
                              className="h-8 rounded-md bg-primary px-3 text-xs font-medium text-primary-foreground disabled:opacity-40">
                              {savingBind[k.id] ? '...' : 'Bind'}
                            </button>
                          </div>
                          <p className="text-[10px] text-muted-foreground">Credits per 1K tokens. 0 = free.</p>
                        </div>
                      </>
                    )}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
