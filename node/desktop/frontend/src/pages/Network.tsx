import { useEffect, useState, useRef, useCallback } from 'react'
import * as api from '../api/backend'
import { useNetworkStore, useToast } from '../stores/nodeStore'

type Tab = 'requests' | 'tasks' | 'node'

interface TaskEvent {
  id: number
  type: 'info' | 'success' | 'warning' | 'error'
  action: string
  detail: string
  ts: number
}

let taskId = 0

export default function Network() {
  const { connected, walletAddress, walletName, nodeName, uptimeSec, peerCount, sharedCount, inputTokens, outputTokens, hasWallet, autoJoin, setStatus, setLoading } = useNetworkStore()
  const addToast = useToast((s) => s.addToast)
  const [tab, setTab] = useState<Tab>('requests')
  const [requests, setRequests] = useState<any[]>([])
  const [connectLoading, setConnectLoading] = useState(false)
  const [autoConnecting, setAutoConnecting] = useState(false)
  const [showLeaveConfirm, setShowLeaveConfirm] = useState(false)
  const [leaveLoading, setLeaveLoading] = useState(false)
  const [taskEvents, setTaskEvents] = useState<TaskEvent[]>([])
  const [showBackup, setShowBackup] = useState(false)
  const [backupSeed, setBackupSeed] = useState('')
  const [backupLoading, setBackupLoading] = useState(false)
  const [copiedAddr, setCopiedAddr] = useState(false)
  const connecting = connectLoading || autoConnecting
  const autoConnectAttempted = useRef(false)

  const handleCopyAddr = () => {
    if (!walletAddress) return
    navigator.clipboard.writeText(walletAddress).then(() => { setCopiedAddr(true); setTimeout(() => setCopiedAddr(false), 2000) })
  }

  const handleBackup = async () => {
    setBackupLoading(true)
    const r = await api.exportSeed()
    setBackupLoading(false)
    if (r?.seed_hex) {
      setBackupSeed(r.seed_hex)
      setShowBackup(true)
      addEvent('info', 'Backup', 'Wallet seed exported')
    } else {
      addToast({ type: 'error', message: r?.error || 'Export seed failed' })
    }
  }

  const addEvent = useCallback((type: TaskEvent['type'], action: string, detail: string) => {
    const e: TaskEvent = { id: ++taskId, type, action, detail, ts: Date.now() }
    setTaskEvents(prev => [e, ...prev].slice(0, 100))
  }, [])

  const load = () => {
    setLoading(true)
    api.networkStatus().then(setStatus)
  }

  // Initial load + 10s status poll
  useEffect(() => {
    load()
    const iv = setInterval(load, 10000)
    return () => clearInterval(iv)
  }, [])

  // Requests tab: 5s poll
  useEffect(() => {
    if (tab !== 'requests') return
    const fetchReqs = () => {
      api.getRecentRequests(50).then(setRequests)
    }
    fetchReqs()
    const iv = setInterval(fetchReqs, 5000)
    return () => clearInterval(iv)
  }, [tab])

  // Auto-connect on mount
  useEffect(() => {
    if (autoConnectAttempted.current) return
    if (!hasWallet || !autoJoin) return
    autoConnectAttempted.current = true
    setAutoConnecting(true)
    let attempts = 0
    const maxAttempts = 5
    const doAttempt = () => {
      if (attempts >= maxAttempts) {
        setAutoConnecting(false)
        addToast({ type: 'warning', message: 'Auto-connect failed, try manually' })
        load()
        return
      }
      attempts++
      const delay = attempts * 2
      setTimeout(async () => {
        const r = await api.connectToNetwork()
        if (r === 'ok' || r === 'already connected') {
          setAutoConnecting(false)
          addEvent('success', 'Auto-connect', 'Connected to network')
          addToast({ type: 'success', message: 'Auto-connected' })
          load()
        } else {
          addEvent('warning', 'Auto-connect', `Attempt ${attempts}/${maxAttempts} failed`)
          if (attempts < maxAttempts) doAttempt()
          else { setAutoConnecting(false); addEvent('error', 'Auto-connect', 'All attempts failed'); addToast({ type: 'warning', message: 'Auto-connect failed' }); load() }
        }
      }, delay * 1000)
    }
    doAttempt()
  }, [hasWallet, autoJoin])

  const handleConnect = async () => {
    setConnectLoading(true)
    const r = await api.connectToNetwork()
    setConnectLoading(false)
    if (r === 'ok' || r === 'already connected') {
      addEvent('success', 'Connect', 'Connected to network')
      addToast({ type: 'success', message: 'Connected' })
      useNetworkStore.setState({ connected: true })
      load()
    } else {
      addEvent('error', 'Connect', r || 'Connect failed')
      addToast({ type: 'error', message: r || 'Connect failed' })
    }
  }

  const handleDisconnect = async () => {
    const r = await api.disconnectFromNetwork()
    if (r === 'ok') {
      addEvent('info', 'Disconnect', 'Disconnected from network')
      addToast({ type: 'info', message: 'Disconnected' }); load()
    }
  }

  const handleLeave = async () => {
    setLeaveLoading(true)
    const r = await api.leaveNetwork()
    setLeaveLoading(false)
    setShowLeaveConfirm(false)
    if (r === 'ok') {
      addEvent('warning', 'Leave Network', 'Wallet deactivated, left network')
      addToast({ type: 'warning', message: 'Left network — wallet deactivated' })
      setRequests([])
      load()
    } else {
      addEvent('error', 'Leave Network', r || 'Leave failed')
      addToast({ type: 'error', message: r || 'Leave failed' })
    }
  }

  // Join page when no wallet
  if (!hasWallet && !connected) {
    return <JoinPage />
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>Network</h1>
          <p>Platform token sharing network</p>
        </div>
        {connected ? (
          <button className="btn btn-secondary btn-sm" onClick={handleDisconnect} style={{ alignSelf: 'flex-start' }}>Disconnect</button>
        ) : (
          <button className="btn btn-primary btn-sm" onClick={handleConnect} disabled={connecting} style={{ alignSelf: 'flex-start' }}>
            {connecting ? 'Connecting...' : 'Connect'}
          </button>
        )}
      </div>

      {/* Status cards */}
      <div className="stat-grid">
        <div className="stat-card">
          <div className="stat-label">Status</div>
          {connected ? (
            <>
              <div><span className="stat-dot green" /><span className="stat-value" style={{ fontSize: 16, verticalAlign: 'middle' }}>Connected</span></div>
              <div className="stat-sub">{uptimeSec ? `${Math.floor(uptimeSec / 60)}m uptime` : '—'}</div>
            </>
          ) : (
            <>
              <div><span className="stat-dot red" /><span className="stat-value" style={{ fontSize: 16, verticalAlign: 'middle' }}>Disconnected</span></div>
              <div className="stat-sub">Not connected to platform</div>
            </>
          )}
        </div>
        <div className="stat-card">
          <div className="stat-label">Wallet</div>
          <div className="stat-value mono" style={{ fontSize: 13 }}>{walletAddress?.slice(0, 16) || '—'}...</div>
          {walletName && <div className="stat-sub">{walletName}</div>}
        </div>
        <div className="stat-card">
          <div className="stat-label">Tokens</div>
          <div className="stat-value" style={{ fontSize: 20 }}>{(inputTokens + outputTokens).toLocaleString()}</div>
          <div className="stat-sub">↑ {(inputTokens || 0).toLocaleString()} in · ↓ {(outputTokens || 0).toLocaleString()} out</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Shared Models</div>
          <div className="stat-value">{sharedCount || 0}</div>
          <div className="stat-sub">registered</div>
        </div>
      </div>

      {/* Tabs */}
      <div className="tabs" style={{ marginBottom: 16 }}>
        <button className={`tab ${tab === 'requests' ? 'active' : ''}`} onClick={() => setTab('requests')}>Requests</button>
        <button className={`tab ${tab === 'tasks' ? 'active' : ''}`} onClick={() => setTab('tasks')}>Tasks</button>
        <button className={`tab ${tab === 'node' ? 'active' : ''}`} onClick={() => setTab('node')}>Node</button>
      </div>

      {/* Tab content */}
      {tab === 'requests' && <RequestsTab requests={requests} />}
      {tab === 'tasks' && <TasksTab events={taskEvents} />}
      {tab === 'node' && (
        <NodeTab
          nodeName={nodeName}
          walletName={walletName}
          walletAddress={walletAddress}
          sharedCount={sharedCount}
          copiedAddr={copiedAddr}
          onCopyAddr={handleCopyAddr}
          onBackup={handleBackup}
          onLeave={() => setShowLeaveConfirm(true)}
        />
      )}

      {/* Backup seed modal */}
      {showBackup && (
        <div className="modal-overlay" onClick={() => setShowBackup(false)}>
          <div className="modal" style={{ maxWidth: 520 }}>
            <div className="modal-header"><h3>Backup Wallet Seed</h3></div>
            <div className="modal-body">
              <div className="warning-banner" style={{ background: '#fff3cd', color: '#856404', padding: 12, borderRadius: 8, marginBottom: 16, fontSize: 13 }}>
                ⚠️ <strong>This seed controls your node identity and earnings.</strong><br />
                Lost seed = lost access forever. Save it somewhere safe — offline, encrypted.
              </div>
              <div className="key-display" style={{ fontSize: 12, userSelect: 'all' }}>{backupSeed}</div>
              <p style={{ fontSize: 12, color: 'var(--c-text-tertiary)', marginTop: 12 }}>
                Wallet: <code>{walletAddress?.slice(0, 24)}...</code>
              </p>
            </div>
            <div className="modal-footer">
              <button className="btn btn-primary" onClick={() => { navigator.clipboard.writeText(backupSeed); addToast({ type: 'success', message: 'Seed copied to clipboard' }) }}>
                📋 Copy Seed
              </button>
              <button className="btn btn-secondary" onClick={() => setShowBackup(false)}>Close</button>
            </div>
          </div>
        </div>
      )}

      {/* Leave confirm modal */}
      {showLeaveConfirm && (
        <div className="modal-overlay" onClick={() => !leaveLoading && setShowLeaveConfirm(false)}>
          <div className="modal" style={{ maxWidth: 450 }}>
            <div className="modal-header"><h3>Leave Network?</h3></div>
            <div className="modal-body">
              <div className="warning-banner" style={{ background: '#fff3cd', color: '#856404', padding: 12, borderRadius: 8, marginBottom: 16, fontSize: 13 }}>
                ⚠️ <strong>This permanently deactivates your wallet.</strong><br /><br />
                Node disconnected. Shared models become unavailable.<br /><br />
                Rejoin later with a new wallet.
              </div>
            </div>
            <div className="modal-footer">
              <button className="btn btn-secondary" onClick={() => setShowLeaveConfirm(false)} disabled={leaveLoading}>Cancel</button>
              <button className="btn btn-danger" onClick={handleLeave} disabled={leaveLoading}>
                {leaveLoading ? 'Leaving...' : 'Yes, Leave Network'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

// ---- Join Page ----
function JoinPage() {
  const [joinName, setJoinName] = useState('')
  const [showSeed, setShowSeed] = useState<any>(null)
  const addToast = useToast((s) => s.addToast)

  const handleCreateWalletAndJoin = async () => {
    const name = joinName.trim() || 'default'
    const wallet = await api.createWallet(name)
    if (wallet.error) { addToast({ type: 'error', message: wallet.error }); return }
    setShowSeed(wallet)
  }

  const handleBackupDone = async () => {
    setShowSeed(null)
    const r = await api.joinNetwork('')
    if (r === 'ok') { addToast({ type: 'success', message: 'Joined network' }) }
    else { addToast({ type: 'error', message: r || 'Join failed' }) }
  }

  return (
    <div>
      <div className="page-header"><h1>Network</h1><p>Platform token sharing network</p></div>
      <div className="card">
        <div className="empty-state">
          <div className="empty-icon">⚡</div>
          <h3>Join the Network</h3>
          <p>Connect to the platform, share your LLM models, and earn credits.</p>
          <p style={{ fontSize: 12, color: 'var(--c-text-tertiary)', marginBottom: 16 }}>
            Fill in a name for your node. A wallet will be created when you join.
          </p>
          <div style={{ display: 'flex', gap: 8, justifyContent: 'center', alignItems: 'center', marginBottom: 16 }}>
            <input
              className="form-input"
              value={joinName}
              onChange={(e) => setJoinName(e.target.value)}
              placeholder="My Home Node"
              style={{ maxWidth: 250 }}
              onKeyDown={(e) => e.key === 'Enter' && joinName.trim() && handleCreateWalletAndJoin()}
            />
            <button className="btn btn-primary" onClick={handleCreateWalletAndJoin} disabled={!joinName.trim()}>
              Join Network
            </button>
          </div>
        </div>
      </div>

      {showSeed && (
        <div className="modal-overlay">
          <div className="modal" style={{ maxWidth: 500 }}>
            <div className="modal-header"><h3>Backup Your Wallet</h3></div>
            <div className="modal-body">
              <div className="warning-banner" style={{ background: '#fff3cd', color: '#856404', padding: 12, borderRadius: 8, marginBottom: 16, fontSize: 13 }}>
                ⚠️ This seed controls your node identity and future earnings.<br />
                <strong>Lost seed = lost access forever.</strong><br />
                Save it somewhere safe.
              </div>
              <div style={{ background: '#f5f5f5', padding: 12, borderRadius: 8, fontFamily: 'monospace', fontSize: 12, wordBreak: 'break-all', marginBottom: 16 }}>
                {showSeed.seed_hex}
              </div>
              <p style={{ fontSize: 13, color: 'var(--c-text-secondary)' }}>
                Wallet address: <code>{showSeed.wallet_address?.slice(0, 20)}...</code>
              </p>
            </div>
            <div className="modal-footer">
              <button className="btn btn-primary" onClick={handleBackupDone}>
                I've Saved the Seed — Join Network
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

// ---- Requests Tab ----
function RequestsTab({ requests }: { requests: any[] }) {
  if (requests.length === 0) {
    return (
      <div className="card">
        <div className="card-body">
          <p style={{ fontSize: 13, color: 'var(--c-text-tertiary)', textAlign: 'center', padding: 20 }}>
            No requests yet. Requests from network consumers will appear here.
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="card">
      <div className="card-header"><h3>Live Requests</h3><span style={{ fontSize: 11, color: 'var(--c-text-tertiary)' }}>auto-refresh 5s</span></div>
      <div style={{ overflowX: 'auto' }}>
        <table style={{ width: '100%', fontSize: 12 }}>
          <thead>
            <tr>
              <th style={{ textAlign: 'left', padding: '8px 10px', borderBottom: '1px solid var(--c-border)' }}>Source</th>
              <th style={{ textAlign: 'left', padding: '8px 10px', borderBottom: '1px solid var(--c-border)' }}>Model</th>
              <th style={{ textAlign: 'right', padding: '8px 10px', borderBottom: '1px solid var(--c-border)' }}>Input Tokens</th>
              <th style={{ textAlign: 'right', padding: '8px 10px', borderBottom: '1px solid var(--c-border)' }}>Output Tokens</th>
              <th style={{ textAlign: 'right', padding: '8px 10px', borderBottom: '1px solid var(--c-border)' }}>Total</th>
              <th style={{ textAlign: 'right', padding: '8px 10px', borderBottom: '1px solid var(--c-border)' }}>Time</th>
              <th style={{ textAlign: 'center', padding: '8px 10px', borderBottom: '1px solid var(--c-border)', width: 60 }}>Status</th>
            </tr>
          </thead>
          <tbody>
            {requests.map((r, i) => (
              <tr key={r.request_id || i}>
                <td style={{ padding: '6px 10px', fontFamily: 'var(--font-mono)', fontSize: 11 }}>
                  {r.source || '—'}
                </td>
                <td style={{ padding: '6px 10px', fontWeight: 500 }}>
                  {r.model_name || r.model_code || r.upstream_model || '—'}
                </td>
                <td style={{ padding: '6px 10px', textAlign: 'right', fontFamily: 'var(--font-mono)', fontSize: 11 }}>
                  {r.input_tokens?.toLocaleString() || '0'}
                </td>
                <td style={{ padding: '6px 10px', textAlign: 'right', fontFamily: 'var(--font-mono)', fontSize: 11 }}>
                  {r.output_tokens?.toLocaleString() || '0'}
                </td>
                <td style={{ padding: '6px 10px', textAlign: 'right', fontFamily: 'var(--font-mono)', fontSize: 11 }}>
                  {r.total_tokens?.toLocaleString() || '0'}
                </td>
                <td style={{ padding: '6px 10px', textAlign: 'right', fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--c-text-tertiary)' }}>
                  {r.created_at ? formatTime(r.created_at) : '—'}
                </td>
                <td style={{ padding: '6px 10px', textAlign: 'center' }}>
                  <span className={`badge ${r.status === 'success' ? 'badge-success' : 'badge-error'}`}>
                    {r.status === 'success' ? 'OK' : r.status || '—'}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

// ---- Node Tab ----
function NodeTab({ nodeName, walletName, walletAddress, sharedCount, copiedAddr, onCopyAddr, onBackup, onLeave }: {
  nodeName: string; walletName: string; walletAddress: string; sharedCount: number; copiedAddr: boolean; onCopyAddr: () => void; onBackup: () => void; onLeave: () => void
}) {
  return (
    <div className="card">
      <div className="card-header"><h3>Node Info</h3></div>
      <div className="card-body">
        <div className="info-grid" style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
          <div>
            <p style={{ fontSize: 11, color: 'var(--c-text-tertiary)', marginBottom: 2 }}>Node Name</p>
            <p style={{ fontSize: 14, fontWeight: 500 }}>{nodeName || walletName || '—'}</p>
          </div>
          <div>
            <p style={{ fontSize: 11, color: 'var(--c-text-tertiary)', marginBottom: 2 }}>Shared Models</p>
            <p style={{ fontSize: 14, fontWeight: 500 }}>{sharedCount} registered</p>
          </div>
          <div style={{ gridColumn: '1 / -1' }}>
            <p style={{ fontSize: 11, color: 'var(--c-text-tertiary)', marginBottom: 2 }}>Wallet Address</p>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
              <span style={{ fontSize: 12, fontFamily: 'var(--font-mono)', wordBreak: 'break-all' }}>{walletAddress?.slice(0, 32) || '—'}{walletAddress?.length > 32 ? '...' : ''}</span>
              <button className="btn btn-ghost btn-sm" onClick={onCopyAddr} title="Copy wallet address" style={{ fontSize: 11 }}>
                {copiedAddr ? '✓ Copied' : '📋 Copy'}
              </button>
              <button className="btn btn-ghost btn-sm" onClick={onBackup} title="Backup private key (seed)" style={{ fontSize: 11 }}>
                🔐 Backup
              </button>
            </div>
          </div>
        </div>
        <div style={{ marginTop: 24, borderTop: '1px solid var(--c-border)', paddingTop: 16 }}>
          <button className="btn btn-danger" onClick={onLeave}>Leave Network</button>
          <p style={{ fontSize: 11, color: 'var(--c-text-tertiary)', marginTop: 6 }}>
            Permanently deactivates wallet and disconnects from platform.
          </p>
        </div>
      </div>
    </div>
  )
}

// ---- Tasks Tab ----
function TasksTab({ events }: { events: TaskEvent[] }) {
  if (events.length === 0) {
    return (
      <div className="card">
        <div className="card-body">
          <p style={{ fontSize: 13, color: 'var(--c-text-tertiary)', textAlign: 'center', padding: 20 }}>
            No tasks yet. Network actions (connect, disconnect, etc.) will appear here.
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="card">
      <div className="card-header"><h3>Recent Tasks</h3></div>
      <div style={{ maxHeight: 420, overflowY: 'auto' }}>
        {events.map((e) => (
          <div key={e.id} style={{ display: 'flex', alignItems: 'flex-start', gap: 12, padding: '10px 20px', borderBottom: '1px solid var(--c-border)', fontSize: 13 }}>
            <span style={{
              width: 8, height: 8, borderRadius: '50%', marginTop: 5, flexShrink: 0,
              background: e.type === 'success' ? 'var(--c-success)' : e.type === 'error' ? 'var(--c-error)' : e.type === 'warning' ? 'var(--c-warning)' : 'var(--c-text-tertiary)'
            }} />
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontWeight: 500 }}>{e.action}</div>
              <div style={{ color: 'var(--c-text-tertiary)', fontSize: 12, marginTop: 1 }}>{e.detail}</div>
            </div>
            <span style={{ fontSize: 11, color: 'var(--c-text-tertiary)', whiteSpace: 'nowrap', flexShrink: 0 }}>{formatTime(e.ts / 1000)}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

function formatTime(ts: number): string {
  const d = new Date(ts * 1000)
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  if (diff < 60000) return 'just now'
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`
  return d.toLocaleDateString()
}
