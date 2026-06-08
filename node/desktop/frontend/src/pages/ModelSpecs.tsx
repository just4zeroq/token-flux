import { useEffect, useState } from 'react'
import * as api from '../api/backend'
import { useModelSpecStore, useToast } from '../stores/nodeStore'

export default function ModelSpecs() {
  const { specs, loading, setSpecs, setLoading } = useModelSpecStore()
  const addToast = useToast((s) => s.addToast)
  const [tab, setTab] = useState<'my' | 'share'>('my')
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
  const platSpecs = specs.filter((s: any) => s.source === 'platform')

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

  const displayList = tab === 'my' ? localSpecs : platSpecs

  return (
    <div>
      <div className="page-header"><h1>Models</h1><p>AI model definitions</p></div>

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <div style={{ display: 'flex', gap: 0 }}>
          <button className={`btn btn-sm ${tab === 'my' ? 'btn-primary' : 'btn-ghost'}`} onClick={() => setTab('my')}>My ({localSpecs.length})</button>
          <button className={`btn btn-sm ${tab === 'share' ? 'btn-primary' : 'btn-ghost'}`} onClick={() => setTab('share')}>Share ({platSpecs.length})</button>
        </div>
        {tab === 'my' && <button className="btn btn-primary" onClick={() => { setForm(blank()); setShowModal(true) }}>+ New Model</button>}
      </div>

      {loading ? <div className="loading"><div className="spinner" /></div> :
      displayList.length === 0 ? <div className="card"><div className="empty-state"><div className="empty-icon">🧠</div><h3>{tab === 'my' ? 'No models' : 'No shared models'}</h3><p>{tab === 'my' ? 'Add a model spec.' : 'Sync from platform in Settings.'}</p></div></div> :
      <div className="table-wrap">
        <table>
          <thead><tr><th style={{width:28}}></th><th>Name</th><th>Developer</th><th>Capabilities</th><th style={{width:60}}>{tab === 'my' ? '' : ''}</th></tr></thead>
          <tbody>
            {displayList.map((s: any) => {
              const expanded = expandId === s.id
              return (
                <tr key={s.id} className={expanded ? 'row-expanded' : ''}>
                  <td style={{textAlign:'center',cursor:'pointer'}} onClick={() => setExpandId(expanded ? null : s.id)}>
                    <span style={{display:'inline-block',transition:'transform .15s',transform: expanded ? 'rotate(90deg)' : ''}}>▶</span>
                  </td>
                  <td style={{fontWeight:500}}>{s.model_name}<span className="mono" style={{marginLeft:8,color:'var(--c-text-tertiary)'}}>{s.model_code}</span></td>
                  <td>{s.developer_name || '—'}</td>
                  <td>{(s.capabilities || []).map((c: string) => <span key={c} className="tag">{c}</span>)}</td>
                  <td>{tab === 'my' ? <button className="btn btn-ghost btn-sm" onClick={() => handleDelete(s.id)} style={{color:'var(--c-error)'}}>Delete</button> : null}</td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>}

      {/* Expanded detail rows */}
      {displayList.map((s: any) => {
        if (expandId !== s.id) return null
        return (
          <div key={`d-${s.id}`} className="card" style={{marginTop:-1,borderTopLeftRadius:0,borderTopRightRadius:0}}>
            <div className="card-body">
              <div className="status-row" style={{flexWrap:'wrap',gap:16}}>
                <span className="status-item"><strong>Context</strong> {s.context_window?.toLocaleString() || '—'}</span>
                <span className="status-item"><strong>Max Input</strong> {s.max_input_tokens?.toLocaleString() || '—'}</span>
                <span className="status-item"><strong>Max Output</strong> {s.max_output_tokens?.toLocaleString() || '—'}</span>
                <span className="status-item"><strong>Model Family</strong> {s.model_family || '—'}</span>
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
