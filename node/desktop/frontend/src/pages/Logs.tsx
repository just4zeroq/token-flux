import { useEffect, useState } from 'react'
import * as api from '../api/backend'

export default function Logs() {
  const [logs, setLogs] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    setLoading(true)
    api.getUsageLogs(100).then((l) => { setLogs(l); setLoading(false) })
    const i = setInterval(() => api.getUsageLogs(100).then(setLogs), 5000)
    return () => clearInterval(i)
  }, [])

  if (loading) return <div className="loading"><div className="spinner" /></div>

  return (
    <div>
      <div className="page-header"><h1>Logs</h1><p>Real-time request log (auto-refreshes)</p></div>

      {logs.length === 0
        ? <div className="card"><div className="empty-state"><div className="empty-icon">📋</div><h3>No logs</h3><p>Send a request to see it here.</p></div></div>
        : <div className="table-wrap">
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
                    <td style={{ fontSize: 12, color: 'var(--c-text-tertiary)', whiteSpace: 'nowrap' }}>{l.created_at ? new Date(l.created_at * 1000).toLocaleString() : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>}
    </div>
  )
}
