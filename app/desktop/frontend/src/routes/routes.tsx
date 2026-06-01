import { createRootRoute, Outlet, Link, Route, Router } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { backend } from '../api/backend'
import { useNodeStore, useKeyStore, useUsageStore, useComboStore, useSettingsStore } from '../stores/nodeStore'

// ===================== Root Layout =====================

const TABS = [
  { id: 'dashboard', label: 'Dashboard', href: '/dashboard' },
  { id: 'keys', label: 'API Keys', href: '/keys' },
  { id: 'combos', label: 'Combos', href: '/combos' },
  { id: 'providers', label: 'Providers', href: '/providers' },
  { id: 'usage', label: 'Usage', href: '/usage' },
  { id: 'logs', label: 'Logs', href: '/logs' },
  { id: 'settings', label: 'Settings', href: '/settings' },
]

const rootRoute = createRootRoute({
  component: () => (
    <div className="flex h-screen bg-gray-50">
      <aside className="w-48 shrink-0 border-r bg-white p-4 flex flex-col">
        <div className="flex items-center gap-2 mb-6 px-2">
          <div className="h-7 w-7 rounded bg-blue-600 text-white text-xs font-bold flex items-center justify-center">T</div>
          <span className="font-semibold text-sm">Token Flux</span>
        </div>
        <nav className="space-y-1 flex-1">
          {TABS.map((t) => (
            <Link key={t.id} to={t.href} className="block px-3 py-2 rounded-md text-sm transition-colors [&.active]:bg-blue-50 [&.active]:text-blue-700 text-gray-600 hover:bg-gray-100">
              {t.label}
            </Link>
          ))}
        </nav>
      </aside>
      <main className="flex-1 overflow-y-auto p-6">
        <Outlet />
      </main>
    </div>
  ),
})

// ===================== Pages =====================

function DashboardPage() {
  const { status, setStatus } = useNodeStore()
  useEffect(() => {
    backend.call('Status').then(setStatus)
    const i = setInterval(() => backend.call('Status').then(setStatus), 3000)
    return () => clearInterval(i)
  }, [])

  const s = status
  return (
    <div>
      <h1 className="text-xl font-bold mb-6">Dashboard</h1>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 mb-8">
        <div className="rounded-xl border bg-white p-4">
          <p className="text-xs text-gray-500">Status</p>
          <div className="flex items-center gap-2 mt-1">
            <span className={`h-2.5 w-2.5 rounded-full ${s?.running ? 'bg-green-500' : 'bg-gray-400'}`} />
            <span className={`font-semibold ${s?.running ? 'text-green-600' : 'text-gray-500'}`}>{s?.running ? 'Running' : 'Stopped'}</span>
          </div>
        </div>
        <div className="rounded-xl border bg-white p-4">
          <p className="text-xs text-gray-500">Uptime</p>
          <p className="text-2xl font-bold mt-1">{s ? Math.floor(s.uptime_sec / 60) + 'm' : '—'}</p>
        </div>
        <div className="rounded-xl border bg-white p-4">
          <p className="text-xs text-gray-500">Models</p>
          <p className="text-2xl font-bold mt-1">{s?.models_count ?? '—'}</p>
        </div>
        <div className="rounded-xl border bg-white p-4">
          <p className="text-xs text-gray-500">API Keys</p>
          <p className="text-2xl font-bold mt-1">{s?.keys_count ?? '—'}</p>
        </div>
      </div>
      <div className="rounded-xl border bg-white p-5">
        <p className="text-xs text-gray-500 mb-2">Endpoints</p>
        <code className="text-sm font-mono bg-gray-100 px-3 py-1.5 rounded block">http://localhost:20128/v1/chat/completions</code>
        <code className="text-sm font-mono bg-gray-100 px-3 py-1.5 rounded block mt-1">http://localhost:20128/api/node/status</code>
      </div>
    </div>
  )
}

function KeysPage() {
  const { keys, setKeys } = useKeyStore()
  const [loading, setLoading] = useState(true)
  const load = async () => {
    const r = await backend.call('ListKeys')
    if (r) setKeys(r)
    setLoading(false)
  }
  useEffect(() => { load() }, [])

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-bold">API Keys</h1>
      </div>
      {loading ? <p className="text-sm text-gray-500">Loading...</p> : keys.length === 0 ? (
        <p className="text-sm text-gray-500">No keys configured. Add keys via the node API.</p>
      ) : (
        <div className="space-y-2">
          {keys.map((k: any) => (
            <div key={k.id} className="rounded-lg border bg-white px-4 py-3 text-sm flex items-center justify-between">
              <div><p className="font-medium">{k.label || 'Unnamed'}</p><p className="text-xs text-gray-500">{k.channel_id} · {k.base_url}</p></div>
              <span className={`text-xs px-2 py-0.5 rounded-full ${k.status === 'active' ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'}`}>{k.status}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function CombosPage() {
  const { combos, setCombos } = useComboStore()
  const load = async () => {
    const r = await backend.call('ListCombos')
    if (r) setCombos(r)
  }
  useEffect(() => { load() }, [])

  return (
    <div>
      <h1 className="text-xl font-bold mb-6">Model Combos</h1>
      {combos.map((c: any) => (
        <div key={c.name} className="rounded-xl border bg-white p-4 mb-3">
          <p className="font-semibold">{c.name}</p>
          <p className="text-xs text-gray-500 mb-2">Strategy: {c.strategy} · Sticky: {c.sticky}</p>
          <div className="flex flex-wrap gap-1.5">{c.models.map((m: string) => (
            <span key={m} className="rounded bg-gray-100 px-2 py-0.5 text-xs">{m}</span>
          ))}</div>
        </div>
      ))}
    </div>
  )
}

function UsagePage() {
  const { stats, setStats } = useUsageStore()
  useEffect(() => { backend.call('GetUsageStats').then(setStats) }, [])

  return (
    <div>
      <h1 className="text-xl font-bold mb-6">Usage</h1>
      <div className="grid gap-4 sm:grid-cols-3 mb-8">
        <div className="rounded-xl border bg-white p-4"><p className="text-xs text-gray-500">Total Calls</p><p className="text-2xl font-bold mt-1">{stats?.total_calls?.toLocaleString() ?? '—'}</p></div>
        <div className="rounded-xl border bg-white p-4"><p className="text-xs text-gray-500">Total Tokens</p><p className="text-2xl font-bold mt-1">{stats?.total_tokens?.toLocaleString() ?? '—'}</p></div>
        <div className="rounded-xl border bg-white p-4"><p className="text-xs text-gray-500">Total Cost</p><p className="text-2xl font-bold mt-1">{stats?.total_cost ?? '—'}</p></div>
      </div>
      {stats?.by_model && stats.by_model.length > 0 && (
        <div className="rounded-xl border bg-white">
          <div className="border-b px-4 py-3"><p className="text-sm font-semibold">By Model</p></div>
          <div className="divide-y text-sm">
            {stats.by_model.map((m) => (
              <div key={m.model} className="flex items-center justify-between px-4 py-2.5">
                <span className="font-medium">{m.model}</span>
                <span className="text-gray-500">{m.calls} calls · {m.tokens.toLocaleString()} tokens</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

function LogsPage() {
  const { logs, setLogs } = useUsageStore()
  useEffect(() => { backend.call('GetUsageLogs', 50).then(setLogs) }, [])

  return (
    <div>
      <h1 className="text-xl font-bold mb-6">Request Logs</h1>
      <div className="rounded-xl border bg-white overflow-hidden">
        {logs.length === 0 ? (
          <p className="p-6 text-sm text-gray-500 text-center">No requests yet.</p>
        ) : (
          <div className="divide-y text-sm">
            {logs.map((l: any, i: number) => (
              <div key={i} className="px-4 py-2.5 flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <span className={`h-2 w-2 rounded-full ${l.success ? 'bg-green-500' : 'bg-red-500'}`} />
                  <span className="font-medium">{l.model}</span>
                  <span className="text-gray-500">{l.tokens} tok</span>
                </div>
                <span className="text-gray-500">{l.latency_ms}ms</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function ProvidersPage() {
  return <div><h1 className="text-xl font-bold mb-6">Providers</h1><p className="text-sm text-gray-500">Provider management coming soon.</p></div>
}

function SettingsPage() {
  const { rtk, rtkMode, caveman, setRTK, setCaveman } = useSettingsStore()

  return (
    <div>
      <h1 className="text-xl font-bold mb-6">Settings</h1>
      <div className="max-w-md space-y-4">
        <div className="rounded-xl border bg-white p-4">
          <label className="flex items-center justify-between">
            <div><p className="font-medium text-sm">RTK Token Saver</p><p className="text-xs text-gray-500">Compress tool results to save tokens</p></div>
            <input type="checkbox" checked={rtk} onChange={(e) => { setRTK(e.target.checked, rtkMode); backend.call('SetRTK', e.target.checked, rtkMode) }} />
          </label>
        </div>
        <div className="rounded-xl border bg-white p-4">
          <label className="flex items-center justify-between">
            <div><p className="font-medium text-sm">Caveman Mode</p><p className="text-xs text-gray-500">Terse prompt injection to save output tokens</p></div>
            <input type="checkbox" checked={caveman} onChange={(e) => { setCaveman(e.target.checked); backend.call('SetCaveman', e.target.checked) }} />
          </label>
        </div>
      </div>
    </div>
  )
}

// ===================== Route Tree =====================

const dashboardRoute = new Route({ getParentRoute: () => rootRoute, path: '/dashboard', component: DashboardPage })
const keysRoute = new Route({ getParentRoute: () => rootRoute, path: '/keys', component: KeysPage })
const combosRoute = new Route({ getParentRoute: () => rootRoute, path: '/combos', component: CombosPage })
const providersRoute = new Route({ getParentRoute: () => rootRoute, path: '/providers', component: ProvidersPage })
const usageRoute = new Route({ getParentRoute: () => rootRoute, path: '/usage', component: UsagePage })
const logsRoute = new Route({ getParentRoute: () => rootRoute, path: '/logs', component: LogsPage })
const settingsRoute = new Route({ getParentRoute: () => rootRoute, path: '/settings', component: SettingsPage })

const routeTree = rootRoute.addChildren([
  dashboardRoute, keysRoute, combosRoute, providersRoute, usageRoute, logsRoute, settingsRoute,
])

const router = new Router({ routeTree })

export { router, rootRoute, dashboardRoute, keysRoute, combosRoute, providersRoute, usageRoute, logsRoute, settingsRoute }
