import { useEffect, useState } from 'react'
import * as api from '../api/backend'
import { useModelSpecStore, useChannelStore, useComboStore, useToast } from '../stores/nodeStore'

type Tab = 'models' | 'channels' | 'combos'

export default function Custom() {
  const [tab, setTab] = useState<Tab>('models')
  return (
    <div>
      <div className="page-header"><h1>Custom</h1><p>Local models, channels, and routing</p></div>
      <div className="tabs" style={{ marginBottom: 20 }}>
        <button className={`tab ${tab === 'models' ? 'active' : ''}`} onClick={() => setTab('models')}>Models</button>
        <button className={`tab ${tab === 'channels' ? 'active' : ''}`} onClick={() => setTab('channels')}>Channels</button>
        <button className={`tab ${tab === 'combos' ? 'active' : ''}`} onClick={() => setTab('combos')}>Combos</button>
      </div>
      {tab === 'models' && <ModelsTab />}
      {tab === 'channels' && <ChannelsTab />}
      {tab === 'combos' && <CombosTab />}
    </div>
  )
}

// ===== Models Tab (local only) =====
function ModelsTab() {
  const { specs, loading, setSpecs, setLoading } = useModelSpecStore()
  const addToast = useToast((s) => s.addToast)
  const [showModal, setShowModal] = useState(false)
  const [expandId, setExpandId] = useState<number | null>(null)

  const blank = () => ({
    developer_name: '', model_name: '', model_code: '', display_name: '', model_family: '',
    description: '', capabilities: [] as string[], context_window: 0,
    max_input_tokens: 0, max_output_tokens: 0,
    supports_stream: true, supports_tools: false, supports_vision: false,
    supports_json_mode: false, supports_reasoning: false, supports_logprobs: false,
  })
  const [form, setForm] = useState<any>(blank())

  const load = () => { setLoading(true); api.listModelSpecs().then(setSpecs) }
  useEffect(() => { load() }, [])

  const localSpecs = specs.filter((s: any) => s.source !== 'platform')

  const handleCreate = async () => {
    const payload = { ...form }
    if (Array.isArray(form.capabilities)) {
      payload.capabilities_json = JSON.stringify(form.capabilities)
    } else if (typeof form.capabilities === 'string' && form.capabilities) {
      payload.capabilities_json = JSON.stringify(form.capabilities.split(',').map((s: string) => s.trim()).filter(Boolean))
    } else {
      payload.capabilities_json = '[]'
    }
    payload.capabilities = undefined
    await api.createModelSpec(payload)
    addToast({ type: 'success', message: 'Model created' })
    setShowModal(false); setForm(blank()); load()
  }

  const handleDelete = async (id: number) => {
    if (!confirm('Delete model?')) return
    if (await api.deleteModelSpec(id)) { addToast({ type: 'success', message: 'Deleted' }); load() }
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 12 }}>
        <button className="btn btn-primary" onClick={() => { setForm(blank()); setShowModal(true) }}>+ New Model</button>
      </div>

      {loading ? <div className="loading"><div className="spinner" /></div> :
      localSpecs.length === 0 ? <div className="card"><div className="empty-state"><div className="empty-icon">🧠</div><h3>No local models</h3><p>Add a model spec to get started.</p></div></div> :
      <div className="table-wrap">
        <table>
          <thead><tr><th style={{width:28}}></th><th>Name</th><th>Developer</th><th>Capabilities</th><th style={{width:60}}></th></tr></thead>
          <tbody>
            {localSpecs.map((s: any) => {
              const expanded = expandId === s.id
              return (
                <tr key={s.id} className={expanded ? 'row-expanded' : ''}>
                  <td style={{textAlign:'center',cursor:'pointer'}} onClick={() => setExpandId(expanded ? null : s.id)}>
                    <span style={{display:'inline-block',transition:'transform .15s',transform: expanded ? 'rotate(90deg)' : ''}}>▶</span>
                  </td>
                  <td style={{fontWeight:500}}>{s.model_name}<span className="mono" style={{marginLeft:8,color:'var(--c-text-tertiary)'}}>{s.model_code}</span></td>
                  <td>{s.developer_name || '—'}</td>
                  <td>{(s.capabilities || []).map((c: string) => <span key={c} className="tag">{c}</span>)}</td>
                  <td><button className="btn btn-ghost btn-sm" onClick={() => handleDelete(s.id)} style={{color:'var(--c-error)'}}>Delete</button></td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>}

      {/* Expanded detail */}
      {localSpecs.map((s: any) => {
        if (expandId !== s.id) return null
        return (
          <div key={`d-${s.id}`} className="card" style={{marginTop:-1,borderTopLeftRadius:0,borderTopRightRadius:0}}>
            <div className="card-body">
              <div className="status-row" style={{flexWrap:'wrap',gap:16}}>
                <span className="status-item"><strong>Context</strong> {s.context_window?.toLocaleString() || '—'}</span>
                <span className="status-item"><strong>Max Input</strong> {s.max_input_tokens?.toLocaleString() || '—'}</span>
                <span className="status-item"><strong>Max Output</strong> {s.max_output_tokens?.toLocaleString() || '—'}</span>
                <span className="status-item"><strong>Family</strong> {s.model_family || '—'}</span>
              </div>
              <div style={{marginTop:8,display:'flex',gap:12,flexWrap:'wrap'}}>
                {s.supports_stream ? <span className="badge badge-active">Stream</span> : null}
                {s.supports_tools ? <span className="badge badge-active">Tools</span> : null}
                {s.supports_vision ? <span className="badge badge-active">Vision</span> : null}
                {s.supports_json_mode ? <span className="badge badge-active">JSON Mode</span> : null}
                {s.supports_reasoning ? <span className="badge badge-active">Reasoning</span> : null}
              </div>
              {s.description ? <p style={{marginTop:8,fontSize:12,color:'var(--c-text-secondary)'}}>{s.description}</p> : null}
              <p style={{marginTop:4,fontSize:11,color:'var(--c-text-tertiary)'}}>Created: {new Date(s.created_at * 1000).toLocaleString()}</p>
            </div>
          </div>
        )
      })}

      {/* Create Modal */}
      {showModal && <div className="modal-overlay" onClick={() => setShowModal(false)}>
        <div className="modal" onClick={(e) => e.stopPropagation()} style={{maxWidth:560}}>
          <div className="modal-header"><h2>New Model</h2><button className="btn btn-ghost btn-sm" onClick={() => setShowModal(false)}>✕</button></div>
          <div className="modal-body">
            <div className="form-row">
              <div className="form-group"><label className="form-label">Model Code</label><input className="form-input" value={form.model_code} onChange={(e) => setForm({...form, model_code: e.target.value})} placeholder="gpt-4o" /></div>
              <div className="form-group"><label className="form-label">Model Name</label><input className="form-input" value={form.model_name} onChange={(e) => setForm({...form, model_name: e.target.value})} placeholder="GPT-4o" /></div>
            </div>
            <div className="form-row">
              <div className="form-group"><label className="form-label">Developer</label><input className="form-input" value={form.developer_name} onChange={(e) => setForm({...form, developer_name: e.target.value})} placeholder="OpenAI" /></div>
              <div className="form-group"><label className="form-label">Family</label><input className="form-input" value={form.model_family} onChange={(e) => setForm({...form, model_family: e.target.value})} placeholder="gpt-4" /></div>
            </div>
            <div className="form-row">
              <div className="form-group"><label className="form-label">Context Window</label><input className="form-input" type="number" value={form.context_window} onChange={(e) => setForm({...form, context_window: Number(e.target.value)})} /></div>
              <div className="form-group"><label className="form-label">Capabilities <span className="optional">(csv)</span></label><input className="form-input" placeholder="chat, vision" onChange={(e) => setForm({...form, capabilities: e.target.value.split(',').map((s: string) => s.trim()).filter(Boolean)})} /></div>
            </div>
            <div className="form-group"><label className="form-label">Description</label><input className="form-input" value={form.description} onChange={(e) => setForm({...form, description: e.target.value})} placeholder="Optional description" /></div>
            <div style={{display:'flex',gap:20}}>
              <label className="toggle"><input type="checkbox" checked={form.supports_stream} onChange={(e) => setForm({...form, supports_stream: e.target.checked})} /><span className="toggle-track"><span className="toggle-thumb" /></span>Stream</label>
              <label className="toggle"><input type="checkbox" checked={form.supports_tools} onChange={(e) => setForm({...form, supports_tools: e.target.checked})} /><span className="toggle-track"><span className="toggle-thumb" /></span>Tools</label>
              <label className="toggle"><input type="checkbox" checked={form.supports_vision} onChange={(e) => setForm({...form, supports_vision: e.target.checked})} /><span className="toggle-track"><span className="toggle-thumb" /></span>Vision</label>
            </div>
          </div>
          <div className="modal-footer">
            <button className="btn btn-secondary" onClick={() => setShowModal(false)}>Cancel</button>
            <button className="btn btn-primary" onClick={handleCreate} disabled={!form.model_code}>Create</button>
          </div>
        </div>
      </div>}
    </div>
  )
}

// ===== Channels Tab (local only) =====
function ChannelsTab() {
  const { channels, loading, setChannels, setLoading } = useChannelStore()
  const addToast = useToast((s) => s.addToast)
  const [expandId, setExpandId] = useState<number | null>(null)
  const [channelModels, setChannelModels] = useState<Record<number, any[]>>({})
  const [showModal, setShowModal] = useState(false)
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

  const loadChannelModels = async (id: number, force = false) => {
    if (!force && channelModels[id]) return
    const list = await api.listChannelModels(id)
    setChannelModels((p) => ({ ...p, [id]: list }))
  }

  const handleCreate = async () => {
    const r = await api.createChannel(code, name, description, providerType, protocols)
    if (r?.error) { addToast({ type: 'error', message: r.error }); return }
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
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 12 }}>
        <button className="btn btn-primary" onClick={() => setShowModal(true)}>+ New Channel</button>
      </div>

      {loading ? <div className="loading"><div className="spinner" /></div> :
      localChannels.length === 0 ? <div className="card"><div className="empty-state"><div className="empty-icon">📡</div><h3>No local channels</h3><p>Add a channel to connect to upstream providers.</p></div></div> :
      <div className="table-wrap">
        <table>
          <thead><tr><th style={{width:28}}></th><th>Name</th><th>Code</th><th>Protocols</th><th>Status</th><th style={{width:60}}></th></tr></thead>
          <tbody>
            {localChannels.map((c: any) => {
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
                  <td><button className="btn btn-ghost btn-sm" onClick={() => handleDelete(c.id)} style={{color:'var(--c-error)'}}>Delete</button></td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>}

      {/* Expanded detail */}
      {localChannels.map((c: any) => {
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
                      <span style={{cursor:'pointer',color:'var(--c-error)',marginLeft:2}} onClick={(e) => { e.stopPropagation(); handleUnbindModel(c.id, m.id) }}>✕</span>
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
              <select className="form-select" value={providerType} onChange={(e) => setProviderType(Number(e.target.value))} style={{width:200}}>
                {PROVIDERS.map((p) => <option key={p.v} value={p.v}>{p.l}</option>)}
              </select>
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
              <p style={{fontSize:11,color:'var(--c-text-tertiary)',marginTop:4}}>Ctrl+click to select multiple</p>
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

// ===== Combos Tab =====
function CombosTab() {
  const { combos, loading, setCombos, setLoading } = useComboStore()
  const addToast = useToast((s) => s.addToast)
  const [showModal, setShowModal] = useState(false)
  const [name, setName] = useState('')
  const [modelsStr, setModelsStr] = useState('')
  const [strategy, setStrategy] = useState('fallback')
  const [sticky, setSticky] = useState(1)

  const load = () => { setLoading(true); api.listCombos().then(setCombos) }
  useEffect(() => { load() }, [])

  const handleCreate = async () => {
    const models = modelsStr.split(',').map((s) => s.trim()).filter(Boolean)
    if (models.length === 0) return
    await api.createCombo(name, models, strategy, sticky)
    addToast({ type: 'success', message: 'Combo created' })
    setShowModal(false); setName(''); setModelsStr(''); setStrategy('fallback'); setSticky(1)
    load()
  }

  const handleDelete = async (n: string) => {
    if (!confirm('Delete combo?')) return
    if (await api.deleteCombo(n)) { addToast({ type: 'success', message: 'Deleted' }); load() }
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 12 }}>
        <button className="btn btn-primary" onClick={() => setShowModal(true)}>+ New Combo</button>
      </div>

      {loading ? <div className="loading"><div className="spinner" /></div> :
      combos.length === 0 ? <div className="card"><div className="empty-state"><div className="empty-icon">🔀</div><h3>No combos</h3><p>Group models with fallback or round-robin strategies.</p></div></div> :
      <div style={{ display: 'grid', gap: 12 }}>
        {combos.map((c: any) => (
          <div key={c.name} className="card">
            <div className="card-header">
              <div><h3>{c.name}</h3></div>
              <button className="btn btn-ghost btn-sm" onClick={() => handleDelete(c.name)} style={{color:'var(--c-error)'}}>Delete</button>
            </div>
            <div className="card-body">
              <div className="status-row" style={{ marginBottom: 12 }}>
                <span className="status-item"><span className="dot green" /> {c.strategy}</span>
                <span className="status-item">sticky: {c.sticky}</span>
              </div>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                {(c.models || []).map((m: string) => <span key={m} className="badge badge-info">{m}</span>)}
              </div>
            </div>
          </div>
        ))}
      </div>}

      {showModal && <div className="modal-overlay" onClick={() => setShowModal(false)}>
        <div className="modal" onClick={(e) => e.stopPropagation()}>
          <div className="modal-header"><h2>New Combo</h2><button className="btn btn-ghost btn-sm" onClick={() => setShowModal(false)}>✕</button></div>
          <div className="modal-body">
            <div className="form-group"><label className="form-label">Name</label><input className="form-input" value={name} onChange={(e) => setName(e.target.value)} placeholder="my-combo" /></div>
            <div className="form-group"><label className="form-label">Models <span className="optional">(comma-separated)</span></label><input className="form-input" value={modelsStr} onChange={(e) => setModelsStr(e.target.value)} placeholder="gpt-4o, claude-sonnet-4" /></div>
            <div className="form-row">
              <div className="form-group"><label className="form-label">Strategy</label><select className="form-select" value={strategy} onChange={(e) => setStrategy(e.target.value)}><option value="fallback">Fallback</option><option value="round-robin">Round Robin</option></select></div>
              <div className="form-group"><label className="form-label">Sticky</label><input className="form-input" type="number" value={sticky} onChange={(e) => setSticky(Number(e.target.value))} min={1} /></div>
            </div>
          </div>
          <div className="modal-footer">
            <button className="btn btn-secondary" onClick={() => setShowModal(false)}>Cancel</button>
            <button className="btn btn-primary" onClick={handleCreate} disabled={!name || !modelsStr}>Create</button>
          </div>
        </div>
      </div>}
    </div>
  )
}
