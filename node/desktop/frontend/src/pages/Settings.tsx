import { useEffect, useState } from 'react'
import * as api from '../api/backend'
import { useSettingsStore, useToast } from '../stores/nodeStore'

export default function Settings() {
  const { autoConnect, loaded, setAutoConnect, setLoaded } = useSettingsStore()
  const addToast = useToast((s) => s.addToast)

  const [platformUrl, setPlatformUrl] = useState('')
  const [cfgPath, setCfgPath] = useState('')
  const [connStatus, setConnStatus] = useState<'ok' | 'fail' | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    api.getConfig().then((cfg) => {
      if (cfg) {
        setAutoConnect(cfg.auto_connect ?? true)
        setPlatformUrl(cfg.platform_url ?? 'http://localhost:8080')
        setCfgPath(cfg.config_path ?? '')
      }
      setLoaded(true)
    })
  }, [])

  const handleSave = async () => {
    setSaving(true)
    const r = await api.saveConfig({ platform_url: platformUrl })
    setSaving(false)
    if (r === 'ok') {
      addToast({ type: 'success', message: 'Config saved' })
    } else {
      addToast({ type: 'error', message: r || 'Save failed' })
    }
  }

  return (
    <div>
      <div className="page-header"><h1>Settings</h1><p>Node configuration</p></div>

      {/* Network */}
      <div className="card" style={{ marginBottom: 16 }}>
        <div className="card-header"><h3>Network</h3></div>
        <div className="card-body" style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <div>
              <p style={{ fontWeight: 500, fontSize: 14 }}>Auto Connect</p>
              <p style={{ fontSize: 12, color: 'var(--c-text-tertiary)' }}>Automatically reconnect to network on app start</p>
            </div>
            <label className="toggle">
              <input type="checkbox" checked={autoConnect} disabled={!loaded} onChange={async (e) => {
                const v = e.target.checked
                setAutoConnect(v)
                await api.saveConfig({ auto_connect: v })
                addToast({ type: 'info', message: v ? 'Auto connect on' : 'Auto connect off' })
              }} />
              <span className="toggle-track"><span className="toggle-thumb" /></span>
            </label>
          </div>
        </div>
      </div>

      {/* Platform Settings */}
      <div className="card" style={{ marginBottom: 16 }}>
        <div className="card-header"><h3>Platform Settings</h3></div>
        <div className="card-body">
          <div className="form-group">
            <label className="form-label">Platform URL</label>
            <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
              <input className="form-input" value={platformUrl} onChange={(e) => setPlatformUrl(e.target.value)} placeholder="http://localhost:8080" style={{ flex: 1 }} />
              <button className="btn btn-secondary btn-sm" disabled={connStatus === 'testing'} onClick={async () => {
                setConnStatus('testing')
                try {
                  const r = await fetch(platformUrl, { method: 'HEAD', signal: AbortSignal.timeout(5000) })
                  setConnStatus(r.ok ? 'ok' : 'fail')
                } catch {
                  setConnStatus('fail')
                }
              }} style={{ whiteSpace: 'nowrap' }}>
                {connStatus === 'testing' ? 'Testing...' : 'Test'}
              </button>
              {connStatus === 'ok' && <span style={{ color: 'var(--c-success)', fontSize: 13, whiteSpace: 'nowrap' }}>✓ Connected</span>}
              {connStatus === 'fail' && <span style={{ color: 'var(--c-error)', fontSize: 13, whiteSpace: 'nowrap' }}>✗ Failed</span>}
            </div>
            <p style={{ fontSize: 11, color: 'var(--c-text-tertiary)', marginTop: 4 }}>Catalog and provider data source. Saved to config.yaml.</p>
          </div>
          <button className="btn btn-primary" onClick={handleSave} disabled={saving} style={{ marginTop: 12 }}>
            {saving ? 'Saving...' : 'Save Config'}
          </button>
          {cfgPath && <p style={{ fontSize: 11, color: 'var(--c-text-tertiary)', marginTop: 8 }}>Config: {cfgPath}</p>}
        </div>
      </div>
    </div>
  )
}
