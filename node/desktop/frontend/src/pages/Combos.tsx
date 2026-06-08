import { useEffect, useState } from 'react'
import * as api from '../api/backend'
import { useComboStore, useToast } from '../stores/nodeStore'

export default function Combos() {
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
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <div><h1>Combos</h1><p>Multi-model routing strategies</p></div>
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
