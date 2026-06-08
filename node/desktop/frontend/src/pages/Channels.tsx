import { useEffect, useState } from 'react'
import * as api from '../api/backend'
import { useChannelStore, useToast } from '../stores/nodeStore'

export default function Channels() {
  const { channels, loading, setChannels, setLoading } = useChannelStore()
  const addToast = useToast((s) => s.addToast)
  const [tab, setTab] = useState<'my' | 'share'>('my')
  const [showModal, setShowModal] = useState(false)
  const [expandId, setExpandId] = useState<number | null>(null)
  const [channelModels, setChannelModels] = useState<Record<number, any[]>>({})
  const [code, setCode] = useState('')
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [providerType, setProviderType] = useState(1)
  const [protocols, setProtocols] = useState([{ protocol: 'openai', base_url: '' }])
  const [modelIds, setModelIds] = useState<number[]>([])
  const [allModels, setAllModels] = useState<any[]>([])

  const load = () => { setLoading(true); api.listChannels().then(setChannels) }
  useEffect(() => { load(); api.listModelSpecs().then(setAllModels) }, [])

  const localChannels = channels.filter((c: any) => c.source !== 'platform')
  const platChannels = channels.filter((c: any) => c.source === 'platform')
  const displayChannels = tab === 'my' ? localChannels : platChannels

  const loadChannelModels = async (id: number, force = false) => {
    if (!force && channelModels[id]) return
    const list = await api.listChannelModels(id)
    console.log('loadChannelModels id=', id, 'returned', list?.length, 'items', list)
    setChannelModels((p) => ({ ...p, [id]: list }))
  }

  const handleCreate = async () => {
    const r = await api.createChannel(code, name, description, providerType, protocols)
    if (r?.error) { addToast({ type: 'error', message: r.error }); return }
    // Bind models if selected
    if (modelIds.length > 0 && r?.id) {
      const ok = await api.bindChannelModels(r.id, modelIds)
      if (!ok) addToast({ type: 'warn', message: 'Channel created but model binding failed' })
    }
    addToast({ type: 'success', message: 'Channel created' })
    setShowModal(false); setCode(''); setName(''); setDescription(''); setProviderType(1)
    setProtocols([{ protocol: 'openai', base_url: '' }]); setModelIds([])
    load()
  }

  const handleDelete = async (id: number) => {
    if (!confirm('Delete channel?')) return
    if (await api.deleteChannel(id)) { addToast({ type: 'success', message: 'Deleted' }); load() }
  }

  const handleUnbindModel = async (channelId: number, bindingId: number) => {
    if (!confirm('Unbind this model from channel?')) return
    const ok = await api.unbindChannelModel(bindingId)
    if (ok) { addToast({ type: 'success', message: 'Unbound' }); loadChannelModels(channelId, true) }
    else { addToast({ type: 'error', message: 'Unbind failed' }) }
  }

  const updateProtocol = (i: number, f: string, v: string) => {
    const p = [...protocols]; (p[i] as any)[f] = v; setProtocols(p)
  }
  const addProtocol = () => setProtocols([...protocols, { protocol: 'openai', base_url: '' }])
  const removeProtocol = (i: number) => setProtocols(protocols.filter((_, x) => x !== i))

  const PROVIDERS = [
    { v: 1, l: 'OpenAI' }, { v: 2, l: 'Claude' }, { v: 3, l: 'Gemini' },
    { v: 4, l: 'Ali' }, { v: 7, l: 'Zhipu' }, { v: 8, l: 'DeepSeek' },
    { v: 9, l: 'Moonshot' }, { v: 10, l: 'Volcengine' }, { v: 15, l: 'Mistral' },
    { v: 16, l: 'xAI' }, { v: 19, l: 'Baidu' }, { v: 22, l: 'Ollama' },
    { v: 25, l: 'SiliconFlow' }, { v: 26, l: 'Xunfei' }, { v: 29, l: 'MiniMax' },
    { v: 6, l: 'Tencent' },
  ]

  return (
    <div>
      <div className="page-header"><h1>Channels</h1><p>Upstream AI provider connections</p></div>

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <div style={{ display: 'flex', gap: 0 }}>
          <button className={`btn btn-sm ${tab === 'my' ? 'btn-primary' : 'btn-ghost'}`} onClick={() => setTab('my')}>My ({localChannels.length})</button>
          <button className={`btn btn-sm ${tab === 'share' ? 'btn-primary' : 'btn-ghost'}`} onClick={() => setTab('share')}>Share ({platChannels.length})</button>
        </div>
        {tab === 'my' && <button className="btn btn-primary" onClick={() => setShowModal(true)}>+ New Channel</button>}
      </div>

      {loading ? <div className="loading"><div className="spinner" /></div> :
      displayChannels.length === 0 ? <div className="card"><div className="empty-state"><div className="empty-icon">📡</div><h3>{tab === 'my' ? 'No channels' : 'No shared channels'}</h3><p>{tab === 'my' ? 'Add a local channel.' : 'Sync from platform.'}</p></div></div> :
      <div className="table-wrap">
        <table>
          <thead><tr><th style={{width:28}}></th><th>Name</th><th>Code</th><th>Protocols</th><th>Status</th><th>Source</th><th style={{width:60}}></th></tr></thead>
          <tbody>
            {displayChannels.map((c: any) => {
              const expanded = expandId === c.id
              return (
                <tr key={c.id} className={expanded ? 'row-expanded' : ''}>
                  <td style={{textAlign:'center',cursor:'pointer'}} onClick={() => { setExpandId(expanded ? null : c.id); if (!expanded) loadChannelModels(c.id) }}>
                    <span style={{display:'inline-block',transition:'transform .15s',transform: expanded ? 'rotate(90deg)' : ''}}>▶</span>
                  </td>
                  <td style={{fontWeight:500}}>{c.name}</td>
                  <td><span className="mono">{c.code}</span></td>
                  <td>{(c.protocols || []).length > 0 ? <span className="badge badge-info">{c.protocols.length} protocol(s)</span> : '—'}</td>
                  <td><span className={`badge badge-${c.status === 'active' ? 'active' : 'disabled'}`}>{c.status}</span></td>
                  <td><span className={`tag ${c.source === 'platform' ? '' : ''}`}>{c.source || 'local'}</span></td>
                  <td>{tab === 'my' ? <button className="btn btn-ghost btn-sm" onClick={() => handleDelete(c.id)} style={{color:'var(--c-error)'}}>Delete</button> : null}</td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>}

      {/* Expanded detail rows */}
      {displayChannels.map((c: any) => {
        if (expandId !== c.id) return null
        const models = channelModels[c.id] || []
        return (
          <div key={`d-${c.id}`} className="card" style={{marginTop:-1,borderTopLeftRadius:0,borderTopRightRadius:0}}>
            <div className="card-body">
              {c.description ? <p style={{marginBottom:8,fontSize:13,color:'var(--c-text-secondary)'}}>{c.description}</p> : null}
              <p style={{fontSize:11,color:'var(--c-text-tertiary)',marginBottom:8}}>Created: {new Date(c.created_at * 1000).toLocaleString()}</p>
              <div>
                <p style={{fontSize:12,fontWeight:500,marginBottom:4}}>Bound Models ({models.length})</p>
                <div style={{display:'flex',flexWrap:'wrap',gap:4}}>
                  {models.length === 0 ? <span style={{fontSize:12,color:'var(--c-text-tertiary)'}}>No models bound</span> :
                  models.map((m: any) => (
                    <span key={m.id} className="tag" style={{display:'inline-flex',alignItems:'center',gap:4}}>
                      {m.model_name || m.model_code || '#' + m.model_spec_id}
                      {tab === 'my' && <span style={{cursor:'pointer',color:'var(--c-error)',marginLeft:2}} onClick={(e) => { e.stopPropagation(); handleUnbindModel(c.id, m.id) }}>✕</span>}
                    </span>
                  ))}
                </div>
              </div>
            </div>
          </div>
        )
      })}

      {/* Create Modal */}
      {showModal && <div className="modal-overlay" onClick={() => setShowModal(false)}>
        <div className="modal" onClick={(e) => e.stopPropagation()} style={{maxWidth:560}}>
          <div className="modal-header"><h2>New Channel</h2><button className="btn btn-ghost btn-sm" onClick={() => setShowModal(false)}>✕</button></div>
          <div className="modal-body">
            <div className="form-group"><label className="form-label">Name</label><input className="form-input" value={name} onChange={(e) => setName(e.target.value)} placeholder="My Channel" /></div>
            <div className="form-group"><label className="form-label">Code</label><input className="form-input" value={code} onChange={(e) => setCode(e.target.value)} placeholder="my-channel" /></div>
            <div className="form-group"><label className="form-label">Description <span className="optional">(optional)</span></label><input className="form-input" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Channel description" /></div>
            <div className="form-group"><label className="form-label">Provider Type</label>
              <div style={{display:'flex',gap:8,flexWrap:'wrap'}}>
                <select className="form-select" value={providerType} onChange={(e) => setProviderType(Number(e.target.value))} style={{width:200}}>
                  {PROVIDERS.map((p) => <option key={p.v} value={p.v}>{p.l}</option>)}
                </select>
              </div>
            </div>
            <div className="form-group"><label className="form-label">Protocols</label>
              {protocols.map((p, i) => (
                <div key={i} style={{display:'flex',gap:8,marginBottom:8}}>
                  <select className="form-select" style={{width:140}} value={p.protocol} onChange={(e) => updateProtocol(i, 'protocol', e.target.value)}>
                    <option value="openai">OpenAI</option>
                    <option value="claude">Claude</option>
                    <option value="gemini">Gemini</option>
                  </select>
                  <input className="form-input" value={p.base_url} onChange={(e) => updateProtocol(i, 'base_url', e.target.value)} placeholder="https://..." />
                  {protocols.length > 1 && <button className="btn btn-ghost btn-sm" onClick={() => removeProtocol(i)} style={{color:'var(--c-error)'}}>✕</button>}
                </div>
              ))}
              <button className="btn btn-ghost btn-sm" onClick={addProtocol} style={{color:'var(--c-accent)'}}>+ Add Protocol</button>
            </div>
            <div className="form-group"><label className="form-label">Bind Models</label>
              <select className="form-select" multiple style={{width:'100%',height:120}} value={modelIds.map(String)} onChange={(e) => setModelIds(Array.from(e.target.selectedOptions, (o) => Number(o.value)))}>
                {allModels.filter((m: any) => m.status === 'active').map((m: any) => (
                  <option key={m.id} value={m.id}>{m.model_name || m.model_code} ({m.developer_name || '—'})</option>
                ))}
              </select>
              <p style={{fontSize:11,color:'var(--c-text-tertiary)',marginTop:4}}>Ctrl+click to select multiple models</p>
            </div>
          </div>
          <div className="modal-footer">
            <button className="btn btn-secondary" onClick={() => setShowModal(false)}>Cancel</button>
            <button className="btn btn-primary" onClick={handleCreate} disabled={!name || !code}>Create</button>
          </div>
        </div>
      </div>}
    </div>
  )
}
