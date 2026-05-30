import { create } from 'zustand'

interface ThemeState {
  theme: string
  setTheme: (t: string) => void
}

const stored = typeof window !== 'undefined' ? localStorage.getItem('theme') : null
const initial = stored || 'purple'
if (typeof document !== 'undefined') {
  document.documentElement.className = `theme-${initial}`
}

export const useThemeStore = create<ThemeState>((set) => ({
  theme: initial,
  setTheme: () => {}, // no-op — single theme
}))
