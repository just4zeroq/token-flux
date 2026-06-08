import { useEffect, useState } from 'react'
import * as api from '../api/backend'
import { useNodeApiKeyStore, useToast } from '../stores/nodeStore'

export default function ApiKeys() {
  const { keys, loading, setKeys, setLoading } = useNodeApiKeyStore()
  const addToast = useToast((s) => s.addToast)
  const [showModal, setShowModal] = useState(false)
  const [label, setLabel] = useState('')
  const [newKeyData, setNewKeyData] = useState<{ key: string; prefix: string } | null>(null)
  const [copiedKey, setCopiedKey] = useState('')

  const load = () => { setLoading(true); api.listNodeApiKeys().then(setKeys) }
  useEffect(() => { load() }, [])

  const handleCreate = async () => {
    const r = await api.createNodeApiKey(label)
    if (r?.key) {
      setNewKeyData({ key: r.key, prefix: r.key_prefix || r.key.slice(0, 12) + '...' })
      addToast({ type: 'success', message: 'API key generated' })
    }
    setLabel('')
    load()
  }

  const handleDelete = async (id: number) => {
    if (!confirm('Revoke this API key?')) return
    if (await api.deleteNodeApiKey(id)) { addToast({ type: 'success', message: 'Key revoked' }); load() }
  }

  const copyToClipboard = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopiedKey(text.slice(0, 16))
      setTimeout(() => setCopiedKey(''), 2000)
      addToast({ type: 'info', message: 'Copied!' })
    } catch { /* fallback */ }
  }

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <div><h1>Gateway API Keys</h1><p>sk-xxx keys for local tools</p></div>
        <button className="btn btn-primary" onClick={() => { setShowModal(true); setNewKeyData(null) }}>+ Generate Key</button>
      </div>

      {newKeyData && (
        <div className="card" style={{ marginBottom: 20, borderColor: '#c7d2fe' }}>
          <div className="card-header"><h3>New Key</h3></div>
          <div className="card-body">
            <div className="key-display" style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
              <span style={{ flex: 1, fontFamily: 'var(--font-mono)', fontSize: 13, wordBreak: 'break-all' }}>{newKeyData.key}</span>
              <button className="btn btn-primary btn-sm" style={{ whiteSpace: 'nowrap', flexShrink: 0 }} onClick={() => copyToClipboard(newKeyData.key)} title="Copy key">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{verticalAlign:'middle',marginRight:4}}><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
                {copiedKey === newKeyData.key.slice(0, 16) ? 'Copied' : 'Copy'}
              </button>
            </div>
            <p style={{ fontSize: 12, color: 'var(--c-text-tertiary)' }}>Store this key securely. It won't be shown again.</p>
          </div>
        </div>
      )}

      {loading ? <div className="loading"><div className="spinner" /></div> :
      keys.length === 0 ? <div className="card"><div className="empty-state"><div className="empty-icon">🔐</div><h3>No gateway keys</h3><p>Generate a key for Claude Code, Codex CLI, or other tools.</p></div></div> :
      <div className="table-wrap">
        <table>
          <thead><tr><th>Label</th><th>Key</th><th>Status</th><th>Last Used</th><th>Created</th><th style={{width:60}}></th></tr></thead>
          <tbody>
            {keys.map((k: any) => (
              <tr key={k.id}>
                <td style={{fontWeight:500}}>{k.label || '—'}</td>
                <td>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                    <span className="mono copy-cell" data-tip={k.key || k.key_prefix}>{k.key_prefix}</span>
                    <button className="btn btn-ghost btn-sm btn-icon" onClick={() => copyToClipboard(k.key || k.key_prefix)} title="Copy key" style={{ padding: 4 }}>
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
                    </button>
                  </div>
                </td>
                <td><span className={`badge badge-${k.status === 'active' ? 'active' : 'disabled'}`}>{k.status}</span></td>
                <td>{k.last_used_at ? new Date(k.last_used_at * 1000).toLocaleString() : 'Never'}</td>
                <td>{new Date(k.created_at * 1000).toLocaleDateString()}</td>
                <td><button className="btn btn-ghost btn-sm" onClick={() => handleDelete(k.id)} style={{color:'var(--c-error)'}}>Revoke</button></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>}

      {showModal && !newKeyData && <div className="modal-overlay" onClick={() => setShowModal(false)}>
        <div className="modal" onClick={(e) => e.stopPropagation()}>
          <div className="modal-header"><h2>Generate API Key</h2><button className="btn btn-ghost btn-sm" onClick={() => setShowModal(false)}>✕</button></div>
          <div className="modal-body">
            <div className="form-group">
              <label className="form-label">Label <span className="optional">(optional)</span></label>
              <input className="form-input" value={label} onChange={(e) => setLabel(e.target.value)} placeholder="Claude Code" autoFocus />
            </div>
          </div>
          <div className="modal-footer">
            <button className="btn btn-secondary" onClick={() => setShowModal(false)}>Cancel</button>
            <button className="btn btn-primary" onClick={handleCreate}>Generate</button>
          </div>
        </div>
      </div>}
    </div>
  )
}
