import { useEffect } from 'react'
import * as api from '../api/backend'
import { useUsageStore } from '../stores/nodeStore'

export default function Usage() {
  const { stats, logs, loading, setStats, setLogs, setLoading } = useUsageStore()

  useEffect(() => {
    setLoading(true)
    Promise.all([
      api.getUsageStats().then((s) => setStats(s ?? {})),
      api.getUsageLogs(100).then(setLogs),
    ])
    const i = setInterval(() => api.getUsageLogs(100).then(setLogs), 5000)
    return () => clearInterval(i)
  }, [])

  if (loading && !stats) return <div className="loading"><div className="spinner" /></div>

  return (
    <div>
      <div className="page-header"><h1>Usage</h1><p>Token consumption and real-time request log</p></div>

      <div className="stat-grid">
        <div className="stat-card">
          <div className="stat-label">Total Calls</div>
          <div className="stat-value mono">{stats?.total_calls?.toLocaleString() ?? '—'}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Success</div>
          <div className="stat-value mono" style={{color:'var(--c-success)'}}>{stats?.success_calls?.toLocaleString() ?? '—'}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Failed</div>
          <div className="stat-value mono" style={{color:'var(--c-error)'}}>{stats?.fail_calls?.toLocaleString() ?? '—'}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Total Tokens</div>
          <div className="stat-value mono">{stats?.total_tokens?.toLocaleString() ?? '—'}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Total Cost</div>
          <div className="stat-value mono">{stats?.total_cost?.toLocaleString() ?? '—'}</div>
          <div className="stat-sub">millicents</div>
        </div>
      </div>

      <div className="card">
        <div className="card-header"><h3>Request Log <span className="badge badge-info" style={{fontSize:11}}>auto-refresh</span></h3></div>
        {logs.length === 0
          ? <div className="card-body"><p style={{color:'var(--c-text-tertiary)',textAlign:'center',padding:16}}>No logs yet. Send a request to see it here.</p></div>
          : <div className="table-wrap" style={{border:0,borderRadius:0,boxShadow:'none'}}>
              <table>
                <thead><tr><th>Status</th><th>Request ID</th><th>Capability</th><th>Tokens</th><th>Cost</th><th>Latency</th><th>Error</th><th>Time</th></tr></thead>
                <tbody>
                  {logs.map((l: any, i: number) => (
                    <tr key={i}>
                      <td><span className={`badge badge-${l.status === 'success' ? 'active' : 'error'}`}>{l.status}</span></td>
                      <td><span className="mono">{(l.request_id || '').slice(0, 16) || '—'}</span></td>
                      <td>{l.capability || 'chat'}</td>
                      <td className="mono">{l.total_tokens?.toLocaleString() ?? '—'}</td>
                      <td className="mono">{l.cost_credits ?? '—'}</td>
                      <td className="mono">{l.latency_ms}ms</td>
                      <td style={{ maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', color: l.error ? 'var(--c-error)' : 'var(--c-text-tertiary)' }}>{l.error || '—'}</td>
                      <td style={{fontSize:12,color:'var(--c-text-tertiary)',whiteSpace:'nowrap'}}>{l.created_at ? new Date(l.created_at * 1000).toLocaleString() : '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>}
      </div>
    </div>
  )
}
