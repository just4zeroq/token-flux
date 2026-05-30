import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'

export const Route = createFileRoute('/desktop')({
  component: DesktopApp,
})

// ---- Tauri bridge ----
interface TauriAPI {
  invoke: (cmd: string, args?: any) => Promise<any>
}

const tauri: TauriAPI | null = (window as any).__TAURI__ ? (window as any).__TAURI__ : null

// ---- Types ----
interface NodeStatus { running: boolean; pid: number | null }

// ---- Components ----

function DesktopApp() {
  const [tab, setTab] = useState('dashboard')
  const [nodeStatus, setNodeStatus] = useState<NodeStatus>({ running: false, pid: null })

  const checkStatus = async () => {
    if (!tauri) return
    try {
      const s: NodeStatus = await tauri.invoke('node_status')
      setNodeStatus(s)
    } catch {}
  }

  useEffect(() => {
    checkStatus()
    const i = setInterval(checkStatus, 3000)
    return () => clearInterval(i)
  }, [])

  const startNode = async () => {
    if (!tauri) return
    try {
      const s: NodeStatus = await tauri.invoke('start_node', {
        config: { platform_url: '', node_id: '', node_secret: '' }
      })
      setNodeStatus(s)
    } catch (e) { alert(String(e)) }
  }

  const stopNode = async () => {
    if (!tauri) return
    try {
      const s: NodeStatus = await tauri.invoke('stop_node')
      setNodeStatus(s)
    } catch (e) { alert(String(e)) }
  }

  const tabs = [
    { id: 'dashboard', label: 'Dashboard' },
    { id: 'keys', label: 'Keys' },
    { id: 'models', label: 'Models' },
    { id: 'settings', label: 'Settings' },
  ]

  return (
    <div className="flex h-screen bg-background overflow-hidden">
      {/* Sidebar */}
      <aside className="w-48 shrink-0 border-r border-border bg-card">
        <div className="px-4 py-4 border-b border-border">
          <div className="flex items-center gap-2">
            <div className="h-6 w-6 rounded bg-primary text-[10px] font-bold text-primary-foreground flex items-center justify-center">N</div>
            <span className="text-sm font-semibold">Node</span>
          </div>
        </div>
        <nav className="p-2 space-y-1">
          {tabs.map((t) => (
            <button
              key={t.id}
              onClick={() => setTab(t.id)}
              className={`w-full text-left px-3 py-2 rounded-md text-sm transition-colors ${
                tab === t.id ? 'bg-primary/10 text-primary font-medium' : 'text-muted-foreground hover:text-foreground hover:bg-muted'
              }`}
            >
              {t.label}
            </button>
          ))}
        </nav>
      </aside>

      {/* Main */}
      <main className="flex-1 overflow-y-auto">
        {tab === 'dashboard' && <DashboardTab nodeStatus={nodeStatus} onStart={startNode} onStop={stopNode} />}
        {tab === 'keys' && <KeysTab />}
        {tab === 'models' && <ModelsTab />}
        {tab === 'settings' && <SettingsTab />}
      </main>
    </div>
  )
}

/* ───── Dashboard Tab ───── */

function DashboardTab({ nodeStatus, onStart, onStop }: {
  nodeStatus: NodeStatus
  onStart: () => void
  onStop: () => void
}) {
  return (
    <div className="p-6 space-y-6">
      <div>
        <h1 className="text-xl font-bold">Node Dashboard</h1>
        <p className="text-sm text-muted-foreground mt-1">Manage your local AI platform node.</p>
      </div>

      {/* Status Card */}
      <div className={`rounded-xl border p-6 ${nodeStatus.running ? 'border-green-500/30 bg-green-500/5' : 'border-border bg-card'}`}>
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-muted-foreground">Node Status</p>
            <div className="flex items-center gap-2 mt-1">
              <span className={`h-2.5 w-2.5 rounded-full ${nodeStatus.running ? 'bg-green-500' : 'bg-muted-foreground'}`} />
              <span className={`text-lg font-bold ${nodeStatus.running ? 'text-green-500' : 'text-muted-foreground'}`}>
                {nodeStatus.running ? 'Running' : 'Stopped'}
              </span>
            </div>
            {nodeStatus.pid && (
              <p className="text-xs text-muted-foreground mt-1">PID: {nodeStatus.pid}</p>
            )}
          </div>
          {nodeStatus.running ? (
            <button onClick={onStop} className="h-9 rounded-lg border border-destructive/30 bg-destructive/10 px-4 text-sm font-medium text-destructive">
              Stop
            </button>
          ) : (
            <button onClick={onStart} className="h-9 rounded-lg bg-primary px-4 text-sm font-medium text-primary-foreground">
              Start Node
            </button>
          )}
        </div>
      </div>

      {/* Stats */}
      <div className="grid gap-4 sm:grid-cols-3">
        <div className="rounded-xl border border-border bg-card p-4">
          <p className="text-xs text-muted-foreground">Models</p>
          <p className="text-2xl font-bold tabular-nums mt-1">—</p>
        </div>
        <div className="rounded-xl border border-border bg-card p-4">
          <p className="text-xs text-muted-foreground">Requests (24h)</p>
          <p className="text-2xl font-bold tabular-nums mt-1">—</p>
        </div>
        <div className="rounded-xl border border-border bg-card p-4">
          <p className="text-xs text-muted-foreground">Avg Latency</p>
          <p className="text-2xl font-bold tabular-nums mt-1">—</p>
        </div>
      </div>

      {/* Endpoints */}
      <div className="rounded-xl border border-border bg-card p-5">
        <h2 className="text-sm font-semibold mb-3">Endpoints</h2>
        <div className="space-y-2 text-sm">
          <div className="flex justify-between">
            <span className="text-muted-foreground">Chat Completions</span>
            <code className="text-xs font-mono">http://localhost:20128/v1/chat/completions</code>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">Admin API</span>
            <code className="text-xs font-mono">http://localhost:20128/api/node/</code>
          </div>
        </div>
      </div>
    </div>
  )
}

/* ───── Keys Tab ───── */

function KeysTab() {
  const [keys, setKeys] = useState<any[]>([])
  const [showAdd, setShowAdd] = useState(false)
  const [label, setLabel] = useState('')
  const [keyValue, setKeyValue] = useState('')
  const [channelID, setChannelID] = useState('')
  const [baseURL, setBaseURL] = useState('')

  const load = async () => {
    try {
      const r = await fetch('http://localhost:20128/api/node/keys')
      const d = await r.json()
      setKeys(d.keys || [])
    } catch {}
  }
  useEffect(() => { load() }, [])

  const add = async () => {
    try {
      await fetch('http://localhost:20128/api/node/keys', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ label, key: keyValue, channel_id: channelID, base_url: baseURL }),
      })
      setShowAdd(false); setLabel(''); setKeyValue(''); setChannelID(''); setBaseURL('')
      load()
    } catch (e) { alert(String(e)) }
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold">API Keys</h1>
          <p className="text-sm text-muted-foreground">Manage your provider API keys.</p>
        </div>
        <button onClick={() => setShowAdd(!showAdd)} className="h-9 rounded-lg bg-primary px-4 text-sm font-medium text-primary-foreground">
          + Add Key
        </button>
      </div>

      {showAdd && (
        <div className="rounded-xl border border-border bg-card p-4 space-y-3">
          <input placeholder="Label" value={label} onChange={(e) => setLabel(e.target.value)} className="w-full h-9 rounded-lg border border-input bg-background px-3 text-sm" />
          <input placeholder="API Key (sk-...)" value={keyValue} onChange={(e) => setKeyValue(e.target.value)} className="w-full h-9 rounded-lg border border-input bg-background px-3 text-sm" />
          <input placeholder="Channel ID" value={channelID} onChange={(e) => setChannelID(e.target.value)} className="w-full h-9 rounded-lg border border-input bg-background px-3 text-sm" />
          <input placeholder="Base URL (optional)" value={baseURL} onChange={(e) => setBaseURL(e.target.value)} className="w-full h-9 rounded-lg border border-input bg-background px-3 text-sm" />
          <button onClick={add} className="h-9 rounded-lg bg-primary px-4 text-sm font-medium text-primary-foreground">Save</button>
        </div>
      )}

      <div className="space-y-2">
        {keys.map((k) => (
          <div key={k.id} className="rounded-lg border border-border bg-card px-4 py-3 text-sm flex items-center justify-between">
            <div>
              <p className="font-medium">{k.label || 'Unnamed'}</p>
              <p className="text-xs text-muted-foreground">{k.channel_id} · {k.base_url}</p>
            </div>
            <span className="text-xs text-muted-foreground">{k.status}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

/* ───── Models Tab ───── */

function ModelsTab() {
  const [bindings, setBindings] = useState<any[]>([])

  const load = async () => {
    try {
      const r = await fetch('http://localhost:20128/api/node/models')
      const d = await r.json()
      setBindings(d.bindings || [])
    } catch {}
  }
  useEffect(() => { load() }, [])

  const [showAdd, setShowAdd] = useState(false)
  const [keyID, setKeyID] = useState('')
  const [modelCode, setModelCode] = useState('')
  const [modelName, setModelName] = useState('')
  const [shared, setShared] = useState(true)

  const add = async () => {
    try {
      await fetch('http://localhost:20128/api/node/models/bind', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key_id: keyID, model_code: modelCode, model_name: modelName, shared }),
      })
      setShowAdd(false); setKeyID(''); setModelCode(''); setModelName('')
      load()
    } catch (e) { alert(String(e)) }
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold">Model Bindings</h1>
          <p className="text-sm text-muted-foreground">Bind local keys to platform models.</p>
        </div>
        <button onClick={() => setShowAdd(!showAdd)} className="h-9 rounded-lg bg-primary px-4 text-sm font-medium text-primary-foreground">
          + Bind Model
        </button>
      </div>

      {showAdd && (
        <div className="rounded-xl border border-border bg-card p-4 space-y-3">
          <input placeholder="Key ID" value={keyID} onChange={(e) => setKeyID(e.target.value)} className="w-full h-9 rounded-lg border border-input bg-background px-3 text-sm" />
          <input placeholder="Model Code (from platform)" value={modelCode} onChange={(e) => setModelCode(e.target.value)} className="w-full h-9 rounded-lg border border-input bg-background px-3 text-sm" />
          <input placeholder="Model Name" value={modelName} onChange={(e) => setModelName(e.target.value)} className="w-full h-9 rounded-lg border border-input bg-background px-3 text-sm" />
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={shared} onChange={(e) => setShared(e.target.checked)} />
            Shared (register to platform)
          </label>
          <button onClick={add} className="h-9 rounded-lg bg-primary px-4 text-sm font-medium text-primary-foreground">Save</button>
        </div>
      )}

      <div className="space-y-2">
        {bindings.map((b) => (
          <div key={b.id} className="rounded-lg border border-border bg-card px-4 py-3 text-sm">
            <div className="flex items-center justify-between">
              <p className="font-medium">{b.model_name || b.model_code}</p>
              <span className={`text-xs px-2 py-0.5 rounded-full ${b.shared ? 'bg-green-500/10 text-green-500' : 'bg-muted text-muted-foreground'}`}>
                {b.shared ? 'Shared' : 'Private'}
              </span>
            </div>
            <p className="text-xs text-muted-foreground mt-1 font-mono">{b.key_hash?.substring(0, 24)}…</p>
          </div>
        ))}
      </div>
    </div>
  )
}

/* ───── Settings Tab ───── */

function SettingsTab() {
  const [platformURL, setPlatformURL] = useState('wss://localhost:8080')
  const [nodeID, setNodeID] = useState('')
  const [nodeSecret, setNodeSecret] = useState('')

  return (
    <div className="p-6 space-y-6">
      <div>
        <h1 className="text-xl font-bold">Settings</h1>
        <p className="text-sm text-muted-foreground">Platform connection settings.</p>
      </div>

      <div className="max-w-md space-y-4">
        <div>
          <label className="text-sm font-medium">Platform URL</label>
          <input value={platformURL} onChange={(e) => setPlatformURL(e.target.value)} className="mt-1 w-full h-9 rounded-lg border border-input bg-card px-3 text-sm" />
        </div>
        <div>
          <label className="text-sm font-medium">Node ID</label>
          <input value={nodeID} onChange={(e) => setNodeID(e.target.value)} className="mt-1 w-full h-9 rounded-lg border border-input bg-card px-3 text-sm" />
        </div>
        <div>
          <label className="text-sm font-medium">Node Secret</label>
          <input type="password" value={nodeSecret} onChange={(e) => setNodeSecret(e.target.value)} className="mt-1 w-full h-9 rounded-lg border border-input bg-card px-3 text-sm" />
        </div>
        <button className="h-9 rounded-lg bg-primary px-4 text-sm font-medium text-primary-foreground">
          Save & Restart Node
        </button>
      </div>
    </div>
  )
}
