import { create } from 'zustand'

interface NodeInfo {
  running: boolean; port: number; uptime_sec: number
  models_count: number; keys_count: number; online: boolean
}

interface UsageStats {
  total_calls: number; total_tokens: number; total_cost: number
  by_model: { model: string; calls: number; tokens: number; cost: number }[]
}

export const useNodeStore = create<{
  status: NodeInfo | null
  setStatus: (s: NodeInfo) => void
}>((set) => ({ status: null, setStatus: (s) => set({ status: s }) }))

export const useKeyStore = create<{
  keys: any[]
  setKeys: (k: any[]) => void
}>((set) => ({ keys: [], setKeys: (k) => set({ keys: k }) }))

export const useUsageStore = create<{
  stats: UsageStats | null
  logs: any[]
  setStats: (s: UsageStats) => void
  setLogs: (l: any[]) => void
}>((set) => ({ stats: null, logs: [], setStats: (s) => set({ stats: s }), setLogs: (l) => set({ logs: l }) }))

export const useComboStore = create<{
  combos: any[]
  setCombos: (c: any[]) => void
}>((set) => ({ combos: [], setCombos: (c) => set({ combos: c }) }))

export const useSettingsStore = create<{
  rtk: boolean; rtkMode: string; caveman: boolean
  setRTK: (b: boolean, m: string) => void
  setCaveman: (b: boolean) => void
}>((set) => ({
  rtk: false, rtkMode: 'auto', caveman: false,
  setRTK: (b, m) => set({ rtk: b, rtkMode: m }),
  setCaveman: (b) => set({ caveman: b }),
}))
