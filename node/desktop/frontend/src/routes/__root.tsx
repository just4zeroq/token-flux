import { createRootRoute, Outlet, Link } from '@tanstack/react-router'
import { useState } from 'react'

export const rootRoute = createRootRoute({
  component: () => {
    const [tab, setTab] = useState('dashboard')

    const tabs = [
      { id: 'dashboard', label: 'Dashboard', href: '/dashboard' },
      { id: 'keys', label: 'Keys', href: '/keys' },
      { id: 'models', label: 'Models', href: '/models' },
      { id: 'settings', label: 'Settings', href: '/settings' },
    ]

    return (
      <div className="flex h-screen bg-gray-50">
        <aside className="w-52 shrink-0 border-r bg-white p-4">
          <div className="flex items-center gap-2 mb-6 px-2">
            <div className="h-7 w-7 rounded bg-blue-600 text-white text-xs font-bold flex items-center justify-center">T</div>
            <span className="font-semibold text-sm">Token Flux</span>
          </div>
          <nav className="space-y-1">
            {tabs.map((t) => (
              <Link
                key={t.id}
                to={t.href}
                onClick={() => setTab(t.id)}
                className="block px-3 py-2 rounded-md text-sm transition-colors [&.active]:bg-blue-50 [&.active]:text-blue-700 text-gray-600 hover:bg-gray-100"
              >
                {t.label}
              </Link>
            ))}
          </nav>
        </aside>
        <main className="flex-1 overflow-y-auto p-6">
          <Outlet />
        </main>
      </div>
    )
  },
})
