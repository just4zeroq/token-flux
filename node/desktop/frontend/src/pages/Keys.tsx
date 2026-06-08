import { useEffect, useState } from 'react'
import * as api from '../api/backend'
import { useKeyStore, useBindingStore, useChannelStore, useNodeApiKeyStore, useToast } from '../stores/nodeStore'

type Tab = 'model' | 'gateway'

export default function Keys() {
  const [tab, setTab] = useState<Tab>('model')
  return (
    <div>
      <div className="page-header"><h1>Keys</h1><p>Upstream provider keys &amp; gateway API keys</p></div>
      <div className="tabs" style={{ marginBottom: 20 }}>
        <button className={`tab ${tab === 'model' ? 'active' : ''}`} onClick={() => setTab('model')}>Model Keys</button>
        <button className={`tab ${tab === 'gateway' ? 'active' : ''}`} onClick={() => setTab('gateway')}>Gateway Keys</button>
      </div>
      {tab === 'model' && <ModelKeysTab />}
      {tab === 'gateway' && <GatewayKeysTab />}
    </div>
  )
}

// ===== Model Keys Tab =====
function ModelKeysTab() {
  const { keys, loading, setKeys, setLoading: kl } = useKeyStore()
  const { bindings, setBindings, setLoading: bl } = useBindingStore()
  const { channels, setChannels, setLoading: cl } = useChannelStore()
  const addToast = useToast((s) => s.addToast)

  const [showCreateModal, setShowCreateModal] = useState(false)
  const [expandKey, setExpandKey] = useState<number | null>(null)

  const [kName, setKName] = useState('')
  const [kValue, setKValue] = useState('')
  const [kChannel, setKChannel] = useState(0)
  const [kBaseUrl, setKBaseUrl] = useState('')
  const [kChannelModels, setKChannelModels] = useState<any[]>([])
  const [selectedModelIds, setSelectedModelIds] = useState<number[]>([])

  const loadAll = () => {
    kl(true); api.listKeys().then(setKeys)
    bl(true); api.listBindings().then(setBindings)
    cl(true); api.listChannels().then(setChannels)
  }
  useEffect(() => { loadAll() }, [])

  useEffect(() => {
    if (!kChannel) { setKChannelModels([]); return }
    api.listChannelModels(kChannel).then((list) => {
      setKChannelModels(list)
      setSelectedModelIds((prev) => prev.filter((id) => list.some((m: any) => m.model_spec_id === id)))
    })
  }, [kChannel])

  const resetForm = () => {
    setKName(''); setKValue(''); setKChannel(0); setKBaseUrl(''); setKChannelModels([]); setSelectedModelIds([])
  }

  const handleCreate = async () => {
    if (!kValue) { addToast({ type: 'error', message: 'API key is required' }); return }
    if (!kChannel) { addToast({ type: 'error', message: 'Select a channel' }); return }
    if (selectedModelIds.length === 0) { addToast({ type: 'error', message: 'Select at least one model' }); return }

    const r = await api.addKey(kName, kValue, kChannel, kBaseUrl)
    if (r?.error) { addToast({ type: 'error', message: r.error }); return }
    const keyId = r.id

    let bound = 0
    for (const msid of selectedModelIds) {
      const m = kChannelModels.find((cm: any) => cm.model_spec_id === msid)
      const modelCode = m?.model_code || 'model-' + msid
      const r2 = await api.bindModel(keyId, modelCode, m?.model_name || modelCode, true)
      if (r2?.error) addToast({ type: 'error', message: `Bind ${modelCode} failed: ${r2.error}` })
      else bound++
    }

    addToast({ type: 'success', message: `Model key created with ${bound} binding(s)` })
    setShowCreateModal(false); resetForm()
    loadAll()
  }

  const handleDeleteKey = async (id: number) => {
    if (!confirm('Delete this key and all its bindings?')) return
    if (await api.deleteKey(id)) { addToast({ type: 'success', message: 'Key deleted' }); loadAll() }
  }

  const handleUnbind = async (id: number) => {
    if (await api.unbindModel(id)) { addToast({ type: 'success', message: 'Unbound' }); loadAll() }
  }

  const bindingsByKey: Record<number, any[]> = {}
  for (const b of bindings) {
    const mkid = b.model_key_id
    if (!bindingsByKey[mkid]) bindingsByKey[mkid] = []
    bindingsByKey[mkid].push(b)
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 12 }}>
        <button className="btn btn-primary" onClick={() => { resetForm(); setShowCreateModal(true) }}>+ Add Model Key</button>
      </div>

      {loading ? <div className="loading"><div className="spinner" /></div> :
      keys.length === 0 ? <div className="card"><div className="empty-state"><div className="empty-icon">🔑</div><h3>No model keys</h3><p>Add an upstream provider API key with model bindings.</p></div></div> :
      <div className="table-wrap">
        <table>
          <thead><tr><th style={{width:28}}></th><th>Name</th><th>Key</th><th>Channel</th><th>Bindings</th><th>Status</th><th style={{width:60}}></th></tr></thead>
          <tbody>
            {keys.map((k: any) => {
              const kBindings = bindingsByKey[k.id] || []
              const expanded = expandKey === k.id
              return (
                <tr key={k.id} className={expanded ? 'row-expanded' : ''}>
                  <td style={{textAlign:'center',cursor:'pointer'}} onClick={() => setExpandKey(expanded ? null : k.id)}>
                    <span style={{display:'inline-block',transition:'transform .15s',transform: expanded ? 'rotate(90deg)' : ''}}>▶</span>
                  </td>
                  <td style={{fontWeight:500}}>{k.name || 'Unnamed'}</td>
                  <td><span className="mono">{k.key_masked || '—'}</span></td>
                  <td>{channels.find((c: any) => c.id === k.channel_id)?.name || `#${k.channel_id}`}</td>
                  <td><span className="badge badge-info">{kBindings.length} bound</span></td>
                  <td><span className={`badge badge-${k.status === 'active' ? 'active' : 'disabled'}`}>{k.status}</span></td>
                  <td><button className="btn btn-ghost btn-sm" onClick={() => handleDeleteKey(k.id)} style={{color:'var(--c-error)'}}>Delete</button></td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>}

      {keys.map((k: any) => {
        const kBindings = bindingsByKey[k.id] || []
        if (expandKey !== k.id || kBindings.length === 0) return null
        return (
          <div key={`b-${k.id}`} className="card" style={{marginTop: -1, borderTopLeftRadius: 0, borderTopRightRadius: 0, padding: 0}}>
            <table className="nested-table">
              <thead><tr><th style={{paddingLeft:40}}>Model Code</th><th>Upstream Model</th><th>Status</th><th>Priority</th><th style={{width:60}}></th></tr></thead>
              <tbody>
                {kBindings.map((b: any) => (
                  <tr key={b.id}>
                    <td style={{paddingLeft:40}}><span className="mono" style={{fontWeight:500}}>{b.model_code}</span></td>
                    <td>{b.upstream_model_name || '—'}</td>
                    <td><span className={`badge badge-${b.status === 'active' ? 'active' : 'disabled'}`}>{b.status}</span></td>
                    <td>{b.priority ?? 0}</td>
                    <td><button className="btn btn-ghost btn-sm" onClick={() => handleUnbind(b.id)} style={{color:'var(--c-error)'}}>Unbind</button></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )
      })}

      {showCreateModal && <div className="modal-overlay" onClick={() => setShowCreateModal(false)}>
        <div className="modal" onClick={(e) => e.stopPropagation()} style={{maxWidth:600}}>
          <div className="modal-header"><h2>Add Model Key</h2><button className="btn btn-ghost btn-sm" onClick={() => setShowCreateModal(false)}>✕</button></div>
          <div className="modal-body">
            <div className="form-group"><label className="form-label">Name</label><input className="form-input" value={kName} onChange={(e) => setKName(e.target.value)} placeholder="My OpenAI Key" /></div>
            <div className="form-group"><label className="form-label">API Key</label><input className="form-input" value={kValue} onChange={(e) => setKValue(e.target.value)} placeholder="sk-..." type="password" /></div>
            <div className="form-row">
              <div className="form-group" style={{flex:1}}>
                <label className="form-label">Channel</label>
                <select className="form-select" value={kChannel} onChange={(e) => {
                  const id = Number(e.target.value)
                  setKChannel(id)
                  const ch = channels.find((c: any) => c.id === id)
                  if (ch?.protocols?.[0]?.base_url) setKBaseUrl(ch.protocols[0].base_url)
                }}>
                  <option value={0}>— Select channel —</option>
                  {channels.filter((c: any) => c.status === 'active').map((c: any) => (
                    <option key={c.id} value={c.id}>{c.name}</option>
                  ))}
                </select>
              </div>
              <div className="form-group" style={{flex:1}}>
                <label className="form-label">Base URL</label>
                <input className="form-input" value={kBaseUrl} onChange={(e) => setKBaseUrl(e.target.value)} placeholder="Auto from channel" />
              </div>
            </div>

            {kChannel > 0 && (
              <>
                <hr style={{margin:'16px 0',border:'none',borderTop:'1px solid var(--c-border)'}} />
                <div className="form-group">
                  <label className="form-label">Supported Models ({kChannelModels.length})</label>
                  {kChannelModels.length === 0 ? (
                    <p style={{fontSize:12,color:'var(--c-text-tertiary)'}}>No models bound to this channel. Bind models in Custom → Channels first.</p>
                  ) : (
                    <div style={{maxHeight:200,overflowY:'auto',border:'1px solid var(--c-border)',borderRadius:8,padding:8}}>
                      {kChannelModels.map((m: any) => {
                        const checked = selectedModelIds.includes(m.model_spec_id)
                        return (
                          <label key={m.id} style={{display:'flex',alignItems:'center',gap:8,padding:'6px 8px',cursor:'pointer',borderRadius:4,background: checked ? 'var(--c-surface-2)' : 'transparent'}}>
                            <input type="checkbox" checked={checked} onChange={(e) => {
                              if (e.target.checked) setSelectedModelIds([...selectedModelIds, m.model_spec_id])
                              else setSelectedModelIds(selectedModelIds.filter((id) => id !== m.model_spec_id))
                            }} />
                            <span style={{fontWeight:500}}>{m.model_name || m.model_code}</span>
                            {m.model_name && <span className="mono" style={{fontSize:11,color:'var(--c-text-tertiary)'}}>({m.model_code})</span>}
                          </label>
                        )
                      })}
                    </div>
                  )}
                </div>
              </>
            )}
          </div>
          <div className="modal-footer">
            <button className="btn btn-secondary" onClick={() => setShowCreateModal(false)}>Cancel</button>
            <button className="btn btn-primary" onClick={handleCreate} disabled={!kValue || !kChannel || selectedModelIds.length === 0}>
              Create ({selectedModelIds.length} models)
            </button>
          </div>
        </div>
      </div>}
    </div>
  )
}

// ===== Gateway Keys Tab =====
function GatewayKeysTab() {
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
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 12 }}>
        <button className="btn btn-primary" onClick={() => { setShowModal(true); setNewKeyData(null) }}>+ Generate Key</button>
      </div>

      {newKeyData && (
        <div className="card" style={{ marginBottom: 20, borderColor: '#c7d2fe' }}>
          <div className="card-header"><h3>New Key</h3></div>
          <div className="card-body">
            <div className="key-display" style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
              <span style={{ flex: 1, fontFamily: 'var(--font-mono)', fontSize: 13, wordBreak: 'break-all' }}>{newKeyData.key}</span>
              <button className="btn btn-primary btn-sm" style={{ whiteSpace: 'nowrap', flexShrink: 0 }} onClick={() => copyToClipboard(newKeyData.key)} title="Copy key">
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
