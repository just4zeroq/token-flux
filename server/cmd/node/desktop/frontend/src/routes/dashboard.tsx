import { Route } from '@tanstack/react-router'
import { rootRoute } from './__root'

export const DashboardRoute = new Route({
  getParentRoute: () => rootRoute,
  path: '/dashboard',
  component: () => {
    return (
      <div>
        <h1 className="text-xl font-bold mb-4">Node Dashboard</h1>
        <div className="grid gap-4 sm:grid-cols-3">
          <div className="rounded-xl border bg-white p-4">
            <p className="text-xs text-gray-500">Status</p>
            <div className="flex items-center gap-2 mt-1">
              <span className="h-2.5 w-2.5 rounded-full bg-green-500" />
              <span className="font-semibold text-green-600">Running</span>
            </div>
          </div>
          <div className="rounded-xl border bg-white p-4">
            <p className="text-xs text-gray-500">Models</p>
            <p className="text-2xl font-bold mt-1">2</p>
          </div>
          <div className="rounded-xl border bg-white p-4">
            <p className="text-xs text-gray-500">Endpoint</p>
            <code className="text-xs font-mono mt-1 block">http://localhost:20128/v1</code>
          </div>
        </div>
      </div>
    )
  },
})

export const KeysRoute = new Route({
  getParentRoute: () => rootRoute,
  path: '/keys',
  component: () => {
    return (
      <div>
        <h1 className="text-xl font-bold mb-4">API Keys</h1>
        <p className="text-sm text-gray-500">Manage local provider API keys.</p>
      </div>
    )
  },
})

export const ModelsRoute = new Route({
  getParentRoute: () => rootRoute,
  path: '/models',
  component: () => {
    return (
      <div>
        <h1 className="text-xl font-bold mb-4">Model Bindings</h1>
        <p className="text-sm text-gray-500">Bind local keys to platform models.</p>
      </div>
    )
  },
})

export const SettingsRoute = new Route({
  getParentRoute: () => rootRoute,
  path: '/settings',
  component: () => {
    return (
      <div>
        <h1 className="text-xl font-bold mb-4">Settings</h1>
        <p className="text-sm text-gray-500">Platform connection and node preferences.</p>
      </div>
    )
  },
})
