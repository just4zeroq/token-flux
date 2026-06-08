import { useEffect, useState } from 'react'
import { getStatus, getTunnelUsage } from '../api/backend'
import { useNodeStore, useNetworkStore } from '../stores/nodeStore'

export default function Dashboard() {
  const { status, loading, setStatus } = useNodeStore()
  const { connected } = useNetworkStore()
  const [endpoint] = useState('localhost:20128')
  const [tunnelUsage, setTunnelUsage] = useState<any>(null)

  useEffect(() => {
    getStatus().then(setStatus)
    const i = setInterval(() => getStatus().then(setStatus), 5000)
    return () => clearInterval(i)
  }, [])

  // Load tunnel usage when connected to network
  useEffect(() => {
    if (connected) {
      getTunnelUsage().then(setTunnelUsage)
      const iv = setInterval(() => getTunnelUsage().then(setTunnelUsage), 30000)
      return () => clearInterval(iv)
    } else {
      setTunnelUsage(null)
    }
  }, [connected])

  if (loading && !status) return <div className="loading"><div className="spinner" /></div>

  return (
    <div>
      <div className="page-header">
        <h1>Dashboard</h1>
        <p>Token Flux Node — local AI gateway</p>
      </div>

      <div className="stat-grid">
        <div className="stat-card">
          <div className="stat-label">Status</div>
          <div>
            <span className={`stat-dot ${status?.running ? 'green' : 'gray'}`} />
            <span className="stat-value" style={{ fontSize: 18, verticalAlign: 'middle' }}>
              {status?.running ? 'Running' : 'Stopped'}
            </span>
          </div>
          <div className="stat-sub">{status?.uptime_sec ? `${Math.floor(status.uptime_sec / 60)}m uptime` : '—'}</div>
        </div>

        <div className="stat-card">
          <div className="stat-label">Models</div>
          <div className="stat-value">{status?.models_count ?? status?.models ?? '—'}</div>
          <div className="stat-sub">registered in router</div>
        </div>

        <div className="stat-card">
          <div className="stat-label">Local Calls</div>
          <div className="stat-value mono">{status?.total_calls?.toLocaleString() ?? '—'}</div>
          <div className="stat-sub">since start</div>
        </div>

        <div className="stat-card">
          <div className="stat-label">Tokens</div>
          <div className="stat-value mono">{status?.total_tokens?.toLocaleString() ?? '—'}</div>
          <div className="stat-sub">total processed</div>
        </div>
      </div>

      {/* Network consumption card — only show when connected */}
      {connected && tunnelUsage && (
        <div className="card" style={{ marginBottom: 16 }}>
          <div className="card-header"><h3>Network Consumption</h3></div>
          <div className="card-body">
            <div className="status-row" style={{ marginBottom: 12 }}>
              <span className="status-item"><strong>External Requests</strong> {tunnelUsage.total_requests}</span>
              <span className="status-item"><strong>External Tokens</strong> {(tunnelUsage.total_tokens || 0).toLocaleString()}</span>
            </div>
            {tunnelUsage.model_breakdown?.length > 0 && (
              <table>
                <thead><tr><th>Model</th><th>Requests</th><th>Tokens</th></tr></thead>
                <tbody>
                  {tunnelUsage.model_breakdown.slice(0, 5).map((m: any, i: number) => (
                    <tr key={i}>
                      <td>{m.model_name || m.model_code || 'unknown'}</td>
                      <td>{m.requests}</td>
                      <td>{(m.total_tokens || 0).toLocaleString()}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      )}

      <div className="card" style={{ marginBottom: 16 }}>
        <div className="card-header"><h3>Gateway Endpoint</h3></div>
        <div className="card-body">
          <p style={{ fontSize: 13, color: '#64748b', marginBottom: 8 }}>Use these endpoints from your local tools:</p>
          <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
            <code style={{ flex: 1 }}>http://{endpoint}/v1/chat/completions</code>
            <code style={{ flex: 1 }}>http://{endpoint}/v1/messages</code>
          </div>
        </div>
      </div>
    </div>
  )
}
