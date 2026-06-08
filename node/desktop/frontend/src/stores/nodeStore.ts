import { create } from 'zustand'

export interface Toast { id: number; type: 'success' | 'error' | 'info'; message: string }

let toastId = 0

// ---- Global toast store ----
export const useToast = create<{
  toasts: Toast[]
  addToast: (t: Omit<Toast, 'id'>) => void
  removeToast: (id: number) => void
}>((set) => ({
  toasts: [],
  addToast: (t) => {
    const id = ++toastId
    set((s) => ({ toasts: [...s.toasts, { ...t, id }] }))
    setTimeout(() => set((s) => ({ toasts: s.toasts.filter((x) => x.id !== id) })), 3500)
  },
  removeToast: (id) => set((s) => ({ toasts: s.toasts.filter((x) => x.id !== id) })),
}))

// ---- Node status ----
export const useNodeStore = create<{
  status: any | null
  loading: boolean
  setStatus: (s: any) => void
  setLoading: (b: boolean) => void
}>((set) => ({ status: null, loading: true, setStatus: (s) => set({ status: s, loading: false }), setLoading: (b) => set({ loading: b }) }))

// ---- Keys ----
export const useKeyStore = create<{
  keys: any[]
  loading: boolean
  setKeys: (k: any[]) => void
  setLoading: (b: boolean) => void
}>((set) => ({ keys: [], loading: true, setKeys: (k) => set({ keys: k, loading: false }), setLoading: (b) => set({ loading: b }) }))

// ---- Bindings ----
export const useBindingStore = create<{
  bindings: any[]
  loading: boolean
  setBindings: (b: any[]) => void
  setLoading: (b: boolean) => void
}>((set) => ({ bindings: [], loading: true, setBindings: (b) => set({ bindings: b, loading: false }), setLoading: (b) => set({ loading: b }) }))

// ---- Combos ----
export const useComboStore = create<{
  combos: any[]
  loading: boolean
  setCombos: (c: any[]) => void
  setLoading: (b: boolean) => void
}>((set) => ({ combos: [], loading: true, setCombos: (c) => set({ combos: c, loading: false }), setLoading: (b) => set({ loading: b }) }))

// ---- Usage ----
export const useUsageStore = create<{
  stats: any | null
  logs: any[]
  loading: boolean
  setStats: (s: any) => void
  setLogs: (l: any[]) => void
  setLoading: (b: boolean) => void
}>((set) => ({ stats: null, logs: [], loading: true, setStats: (s) => set({ stats: s, loading: false }), setLogs: (l) => set({ logs: l, loading: false }), setLoading: (b) => set({ loading: b }) }))

// ---- Channels ----
export const useChannelStore = create<{
  channels: any[]
  loading: boolean
  setChannels: (c: any[]) => void
  setLoading: (b: boolean) => void
}>((set) => ({ channels: [], loading: true, setChannels: (c) => set({ channels: c, loading: false }), setLoading: (b) => set({ loading: b }) }))

// ---- Model Specs ----
export const useModelSpecStore = create<{
  specs: any[]
  loading: boolean
  setSpecs: (s: any[]) => void
  setLoading: (b: boolean) => void
}>((set) => ({ specs: [], loading: true, setSpecs: (s) => set({ specs: s, loading: false }), setLoading: (b) => set({ loading: b }) }))

// ---- Node API Keys ----
export const useNodeApiKeyStore = create<{
  keys: any[]
  loading: boolean
  setKeys: (k: any[]) => void
  setLoading: (b: boolean) => void
}>((set) => ({ keys: [], loading: true, setKeys: (k) => set({ keys: k, loading: false }), setLoading: (b) => set({ loading: b }) }))

// ---- Settings ----
export const useSettingsStore = create<{
  autoConnect: boolean
  loaded: boolean
  setAutoConnect: (b: boolean) => void
  setLoaded: (b: boolean) => void
}>((set) => ({
  autoConnect: true, loaded: false,
  setAutoConnect: (b) => set({ autoConnect: b }),
  setLoaded: (b) => set({ loaded: b }),
}))

// ---- Wallet ----
export const useWalletStore = create<{
  hasWallet: boolean
  walletAddress: string
  seedHex: string
  showBackup: boolean
  loading: boolean
  setWallet: (w: any) => void
  setLoading: (b: boolean) => void
}>((set) => ({
  hasWallet: false, walletAddress: '', seedHex: '', showBackup: false, loading: true,
  setWallet: (w) => set({
    hasWallet: w?.has_wallet ?? false,
    walletAddress: w?.wallet_address ?? '',
    showBackup: !!w?.seed_hex,
    seedHex: w?.seed_hex ?? '',
    loading: false,
  }),
  setLoading: (b) => set({ loading: b }),
}))

// ---- Network ----
export const useNetworkStore = create<{
  connected: boolean
  hasWallet: boolean
  walletAddress: string
  walletName: string
  nodeName: string
  uptimeSec: number
  peerCount: number
  sharedCount: number
  inputTokens: number
  outputTokens: number
  autoJoin: boolean
  loading: boolean
  setStatus: (s: any) => void
  setLoading: (b: boolean) => void
}>((set) => ({
  connected: false, hasWallet: false, walletAddress: '', walletName: '', nodeName: '',
  uptimeSec: 0, peerCount: 0, sharedCount: 0, inputTokens: 0, outputTokens: 0, autoJoin: false, loading: true,
  setStatus: (s) => set({
    connected: s?.connected ?? false,
    hasWallet: s?.has_wallet ?? false,
    walletAddress: s?.wallet_address ?? '',
    walletName: s?.wallet_name ?? '',
    nodeName: s?.node_name ?? '',
    uptimeSec: s?.uptime_sec ?? 0,
    peerCount: s?.peer_count ?? 0,
    sharedCount: s?.shared_count ?? 0,
    inputTokens: s?.input_tokens ?? 0,
    outputTokens: s?.output_tokens ?? 0,
    autoJoin: s?.auto_join ?? false,
    loading: false,
  }),
  setLoading: (b) => set({ loading: b }),
}))
