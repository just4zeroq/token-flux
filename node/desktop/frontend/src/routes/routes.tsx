import { createRootRoute, Outlet, Link, Route, Router } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { backend } from '../api/backend'
import { useNodeStore, useKeyStore, useUsageStore, useComboStore } from '../stores/nodeStore'

const TABS = [
  { id: 'dashboard', label: 'Dashboard', href: '/dashboard' },
  { id: 'keys', label: 'Keys', href: '/keys' },
  { id: 'combos', label: 'Combos', href: '/combos' },
  { id: 'usage', label: 'Usage', href: '/usage' },
  { id: 'logs', label: 'Logs', href: '/logs' },
  { id: 'settings', label: 'Settings', href: '/settings' },
]

const rootRoute = createRootRoute({
  component: () => (
    <div className="flex h-screen bg-gray-50">
      <aside className="w-44 shrink-0 border-r bg-white p-4">
        <div className="flex items-center gap-2 mb-6 px-2">
          <div className="h-7 w-7 rounded bg-blue-600 text-white font-bold flex items-center justify-center">T</div>
          <span className="font-semibold">Token Flux</span>
        </div>
        <nav className="space-y-1">
          {TABS.map((t) => (<Link key={t.id} to={t.href} className="block px-3 py-2 rounded-md text-sm [&.active]:bg-blue-50 [&.active]:text-blue-700 text-gray-600 hover:bg-gray-100">{t.label}</Link>))}
        </nav>
      </aside>
      <main className="flex-1 overflow-y-auto p-6"><Outlet /></main>
    </div>
  ),
})

function Dashboard() {
  const { status, setStatus } = useNodeStore()
  useEffect(() => { backend.call('Status').then(setStatus); const i = setInterval(() => backend.call('Status').then(setStatus), 3000); return () => clearInterval(i) }, [])
  return (<div>
    <h1 className="text-xl font-bold mb-6">Dashboard</h1>
    <div className="grid gap-4 sm:grid-cols-3 mb-6">
      <div className="rounded-xl border bg-white p-4"><p className="text-xs text-gray-500">Status</p><div className="flex items-center gap-2 mt-1"><span className={`h-2.5 w-2.5 rounded-full ${status?.running ? 'bg-green-500' : 'bg-gray-400'}`} /><span className={`font-semibold ${status?.running ? 'text-green-600' : 'text-gray-500'}`}>{status?.running ? 'Running' : 'Stopped'}</span></div></div>
      <div className="rounded-xl border bg-white p-4"><p className="text-xs text-gray-500">Models</p><p className="text-2xl font-bold mt-1">{status?.models_count ?? '--'}</p></div>
      <div className="rounded-xl border bg-white p-4"><p className="text-xs text-gray-500">Endpoint</p><code className="text-xs font-mono mt-1 block">localhost:20128</code></div>
    </div>
  </div>)
}

function Keys() { const { keys, setKeys } = useKeyStore(); useEffect(() => { backend.call('ListKeys').then(setKeys) }, [])
  return (<div><h1 className="text-xl font-bold mb-6">API Keys</h1>
    {(!keys || keys.length === 0) ? <p className="text-sm text-gray-500">No keys.</p> :
    <div className="space-y-2">{keys.map((k: any) => (
      <div key={k.id} className="rounded-lg border bg-white px-4 py-3 text-sm"><p className="font-medium">{k.label || 'Unnamed'}</p><p className="text-xs text-gray-500">{k.channel_id} / {k.base_url}</p></div>
    ))}</div>}
  </div>)
}

function Combos() { const { combos, setCombos } = useComboStore(); useEffect(() => { backend.call('ListCombos').then(setCombos) }, [])
  return (<div><h1 className="text-xl font-bold mb-6">Combos</h1>
    {(!combos || combos.length === 0) ? <p className="text-sm text-gray-500">No combos.</p> :
    combos.map((c: any) => (<div key={c.name} className="rounded-xl border bg-white p-4 mb-3"><p className="font-semibold">{c.name}</p><p className="text-xs text-gray-500 mb-2">{c.strategy} / sticky: {c.sticky}</p><div className="flex flex-wrap gap-1">{c.models.map((m: string) => <span key={m} className="rounded bg-gray-100 px-2 py-0.5 text-xs">{m}</span>)}</div></div>))}
  </div>)
}

function Usage() { const { stats, setStats } = useUsageStore(); useEffect(() => { backend.call('GetUsageStats').then(setStats) }, [])
  return (<div><h1 className="text-xl font-bold mb-6">Usage</h1>
    <div className="grid gap-4 sm:grid-cols-3 mb-6">
      <div className="rounded-xl border bg-white p-4"><p className="text-xs text-gray-500">Calls</p><p className="text-2xl font-bold mt-1">{stats?.total_calls?.toLocaleString() ?? '--'}</p></div>
      <div className="rounded-xl border bg-white p-4"><p className="text-xs text-gray-500">Tokens</p><p className="text-2xl font-bold mt-1">{stats?.total_tokens?.toLocaleString() ?? '--'}</p></div>
      <div className="rounded-xl border bg-white p-4"><p className="text-xs text-gray-500">Cost</p><p className="text-2xl font-bold mt-1">{stats?.total_cost ?? '--'}</p></div>
    </div>
  </div>)
}

function Logs() { const { logs, setLogs } = useUsageStore(); useEffect(() => { backend.call('GetUsageLogs', 50).then(setLogs) }, [])
  return (<div><h1 className="text-xl font-bold mb-6">Logs</h1>
    <div className="rounded-xl border bg-white">{(logs || []).length === 0 ? <p className="p-6 text-sm text-gray-500 text-center">No logs.</p> : (
      <div className="divide-y text-sm">{logs.map((l: any, i: number) => (
        <div key={i} className="px-4 py-2.5 flex items-center justify-between">
          <span className={`h-2 w-2 rounded-full ${l.success ? 'bg-green-500' : 'bg-red-500'}`} />
          <span className="font-medium">{l.model}</span>
          <span className="text-gray-500">{l.tokens} tok / {l.latency_ms}ms</span>
        </div>
      ))}</div>)}
    </div>
  </div>)
}

function Settings() { return (<div><h1 className="text-xl font-bold mb-6">Settings</h1><p className="text-sm text-gray-500">RTK, Caveman mode and language settings.</p></div>) }

const dashboardRoute = new Route({ getParentRoute: () => rootRoute, path: '/dashboard', component: Dashboard })
const keysRoute = new Route({ getParentRoute: () => rootRoute, path: '/keys', component: Keys })
const combosRoute = new Route({ getParentRoute: () => rootRoute, path: '/combos', component: Combos })
const usageRoute = new Route({ getParentRoute: () => rootRoute, path: '/usage', component: Usage })
const logsRoute = new Route({ getParentRoute: () => rootRoute, path: '/logs', component: Logs })
const settingsRoute = new Route({ getParentRoute: () => rootRoute, path: '/settings', component: Settings })

export const router = new Router({
  routeTree: rootRoute.addChildren([dashboardRoute, keysRoute, combosRoute, usageRoute, logsRoute, settingsRoute]),
})
