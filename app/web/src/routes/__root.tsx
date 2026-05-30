import { createRootRoute, Outlet } from '@tanstack/react-router'
import { useEffect } from 'react'
import { useAuthStore } from '../stores/auth'

// Initialize theme on module load (before any component renders)
import '../stores/theme'

export const Route = createRootRoute({
  component: RootLayout,
})

function RootLayout() {
  const token = useAuthStore((s) => s.token)
  const user = useAuthStore((s) => s.user)
  const fetchProfile = useAuthStore((s) => s.fetchProfile)

  useEffect(() => {
    if (token && !user) {
      fetchProfile()
    }
  }, [token, user, fetchProfile])

  return (
    <div className="min-h-screen bg-background text-foreground antialiased">
      <Outlet />
    </div>
  )
}
