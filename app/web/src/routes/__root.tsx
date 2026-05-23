import { createRootRoute, Outlet, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { useAuthStore } from '../stores/auth'

export const Route = createRootRoute({
  component: RootLayout,
})

function RootLayout() {
  const token = useAuthStore((s) => s.token)
  const user = useAuthStore((s) => s.user)
  const fetchProfile = useAuthStore((s) => s.fetchProfile)
  const logout = useAuthStore((s) => s.logout)
  const navigate = useNavigate()

  useEffect(() => {
    if (token && !user) {
      fetchProfile()
    }
  }, [token, user, fetchProfile])

  const path = window.location.pathname
  const isLoginPage = path === '/login'

  return (
    <div className="min-h-screen bg-background">
      {token && !isLoginPage && (
        <header className="border-b px-6 py-3 flex items-center justify-between">
          <div className="flex items-center gap-6">
            <h2 className="font-semibold">AI Platform</h2>
            <nav className="flex gap-4 text-sm">
              <a href="/dashboard" className="text-muted-foreground hover:text-foreground">Dashboard</a>
              <a href="/keys" className="text-muted-foreground hover:text-foreground">API Keys</a>
              <a href="/models" className="text-muted-foreground hover:text-foreground">Models</a>
              <a href="/usage" className="text-muted-foreground hover:text-foreground">Usage</a>
              <a href="/orders" className="text-muted-foreground hover:text-foreground">Orders</a>
              <a href="/transactions" className="text-muted-foreground hover:text-foreground">Transactions</a>
            </nav>
          </div>
          <div className="flex items-center gap-3 text-sm">
            <span className="text-muted-foreground">{user?.username}</span>
            <button onClick={() => { logout(); navigate({ to: '/login' })}} className="text-muted-foreground hover:text-foreground">
              Logout
            </button>
          </div>
        </header>
      )}
      <Outlet />
    </div>
  )
}
