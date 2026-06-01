import { Route, Router } from '@tanstack/react-router'
import { rootRoute } from './routes/__root'
import { DashboardRoute } from './routes/dashboard'
import { KeysRoute } from './routes/keys'
import { ModelsRoute } from './routes/models'
import { SettingsRoute } from './routes/settings'

const routeTree = rootRoute.addChildren([
  DashboardRoute,
  KeysRoute,
  ModelsRoute,
  SettingsRoute,
])

const router = new Router({ routeTree })

export { router }
