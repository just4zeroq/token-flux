import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import * as api from '../api/backend'
import { useWalletStore } from '../stores/nodeStore'

export default function AuthWidget() {
  const { hasWallet, setWallet } = useWalletStore()
  const navigate = useNavigate()

  useEffect(() => {
    api.walletStatus().then((r) => {
      if (r) setWallet(r)
    })
  }, [])

  if (!hasWallet) {
    return (
      <button className="btn btn-ghost btn-sm" onClick={() => navigate('/network')} style={{ fontSize: 12 }}>
        ⊞ Setup Wallet
      </button>
    )
  }

  return null
}
