import { useEffect, useState } from 'react'
import * as api from '../api/backend'
import { useWalletStore, useToast } from '../stores/nodeStore'

interface Props {
  onDone: () => void
  onClose: () => void
}

export default function SetupWallet({ onDone, onClose }: Props) {
  const { setWallet } = useWalletStore()
  const addToast = useToast((s) => s.addToast)
  const [step, setStep] = useState<'generating' | 'backup' | 'done'>('generating')
  const [result, setResult] = useState<any>(null)

  const generate = async () => {
    setStep('generating')
    const r = await api.setupWallet()
    if (r?.error) {
      addToast({ type: 'error', message: r.error })
      onClose()
      return
    }
    setResult(r)
    setWallet(r)
    setStep('backup')
  }

  const copySeed = () => {
    if (result?.seed_hex) {
      navigator.clipboard.writeText(result.seed_hex)
      addToast({ type: 'info', message: 'Seed copied' })
    }
  }

  const downloadBackup = () => {
    if (!result?.seed_hex) return
    const blob = new Blob([
      'Token Flux Node — Wallet Backup\n',
      '==============================\n\n',
      `Wallet Address: ${result.wallet_address}\n`,
      `Seed (hex):     ${result.seed_hex}\n\n`,
      `Generated on:   ${new Date().toISOString()}\n`,
      `Host:           ${result.host || '(unknown)'}\n\n`,
      '⚠️  KEEP THIS SEED SECRET. Anyone with it controls your node.\n',
      '⚠️  LOSE IT = FOREVER LOSE ACCESS to your node and earnings.\n',
    ], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url; a.download = `flux-wallet-${result.wallet_address?.slice(0, 8)}.txt`
    a.click()
    URL.revokeObjectURL(url)
  }

  useEffect(() => { generate() }, [])

  if (step === 'generating') {
    return (
      <div className="modal-overlay">
        <div className="modal" style={{ maxWidth: 420 }}>
          <div className="modal-body" style={{ textAlign: 'center', padding: 40 }}>
            <div className="spinner" style={{ margin: '0 auto 16px' }} />
            <p>Generating wallet...</p>
          </div>
        </div>
      </div>
    )
  }

  if (step === 'backup') {
    return (
      <div className="modal-overlay">
        <div className="modal" style={{ maxWidth: 520 }}>
          <div className="modal-header">
            <h2>⚠️ Backup Your Wallet Seed</h2>
          </div>
          <div className="modal-body">
            <div style={{ background: '#fef2f2', borderRadius: 8, padding: 12, marginBottom: 16, fontSize: 12, color: '#991b1b' }}>
              <strong>Critical:</strong> This seed controls your node identity and earnings.
              Lost seed = permanent loss. Anyone with it controls your node.
              Write it down or store it securely offline.
            </div>

            <div className="form-group">
              <label className="form-label">Wallet Address</label>
              <div className="key-display" style={{ fontSize: 11, wordBreak: 'break-all' }}>{result?.wallet_address}</div>
            </div>

            <div className="form-group">
              <label className="form-label">Private Key Seed (hex)</label>
              <div className="key-display" style={{ fontSize: 11, wordBreak: 'break-all', userSelect: 'all' }}>{result?.seed_hex}</div>
            </div>

            <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
              <button className="btn btn-secondary btn-sm" onClick={copySeed}>Copy Seed</button>
              <button className="btn btn-secondary btn-sm" onClick={downloadBackup}>Download Backup</button>
            </div>

            <p style={{ fontSize: 12, color: 'var(--c-text-tertiary)' }}>
              Make sure you've saved the seed before continuing.
            </p>
          </div>
          <div className="modal-footer">
            <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
            <button className="btn btn-primary" onClick={() => onDone()}>
              Confirm & Continue
            </button>
          </div>
        </div>
      </div>
    )
  }

  return null
}
