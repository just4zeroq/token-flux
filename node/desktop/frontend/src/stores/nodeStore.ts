import { create } from 'zustand'
export const useNodeStore = create<{
  status: any | null; setStatus: (s: any) => void
}>((set) => ({ status: null, setStatus: (s) => set({ status: s }) }))

export const useKeyStore = create<{
  keys: any[]; setKeys: (k: any[]) => void
}>((set) => ({ keys: [], setKeys: (k) => set({ keys: k }) }))

export const useUsageStore = create<{
  stats: any | null; logs: any[]; setStats: (s: any) => void; setLogs: (l: any[]) => void
}>((set) => ({ stats: null, logs: [], setStats: (s) => set({ stats: s }), setLogs: (l) => set({ logs: l }) }))

export const useComboStore = create<{ combos: any[]; setCombos: (c: any[]) => void }>((set) => ({ combos: [], setCombos: (c) => set({ combos: c }) }))

export const useSettingsStore = create<{ rtk: boolean; caveman: boolean; setRTK: (b: boolean, m?: string) => void; setCaveman: (b: boolean) => void }>((set) => ({
  rtk: false, caveman: false,
  setRTK: (b) => set({ rtk: b }),
  setCaveman: (b) => set({ caveman: b }),
}))
