import { create } from 'zustand'
import { apiGet, apiPost } from '../api/client'

interface User {
  id: number
  username: string
  email: string
  display_name?: string
  avatar?: string
  role?: string
  kyc_status?: string
}

interface AuthState {
  token: string | null
  user: User | null
  loading: boolean
  setAuth: (token: string, user: User) => void
  login: (email: string, password: string) => Promise<void>
  register: (email: string, password: string) => Promise<void>
  fetchProfile: () => Promise<void>
  logout: () => void
}

export const useAuthStore = create<AuthState>((set, get) => ({
  token: localStorage.getItem('token'),
  user: null,
  loading: false,

  setAuth: (token, user) => {
    localStorage.setItem('token', token)
    set({ token, user })
  },

  login: async (email, password) => {
    set({ loading: true })
    try {
      const res = await apiPost<{ token: string; user: User }>(
        '/auth/login', { email, password }
      )
      localStorage.setItem('token', res.token)
      set({ token: res.token, user: res.user, loading: false })
    } catch (e) {
      set({ loading: false })
      throw e
    }
  },

  register: async (email, password) => {
    set({ loading: true })
    try {
      await apiPost<{ message: string }>(
        '/auth/register', { email, password }
      )
      set({ loading: false })
    } catch (e) {
      set({ loading: false })
      throw e
    }
  },

  fetchProfile: async () => {
    const { token } = get()
    if (!token) return
    try {
      const user = await apiGet<User>('/users/me', token)
      set({ user })
    } catch {
      localStorage.removeItem('token')
      set({ token: null, user: null })
    }
  },

  logout: () => {
    localStorage.removeItem('token')
    set({ token: null, user: null })
  },
}))
