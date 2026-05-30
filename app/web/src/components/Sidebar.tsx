import { useAuthStore } from '../stores/auth'

const BASE_ITEMS = [
  { key: 'overview', label: 'Overview', icon: '◉' },
  { key: 'keys', label: 'API Keys', icon: '⚷' },
  { key: 'usage', label: 'Usage', icon: '▤' },
  { key: 'recharge', label: 'Recharge', icon: '⟳' },
]

const PROVIDER_ITEMS = [
  { key: 'channels', label: 'Channels', icon: '⇆' },
  { key: 'provider-models', label: 'Models', icon: '▤' },
  { key: 'provider-keys', label: 'Model Keys', icon: '⚷' },
]

const SETTINGS_ITEM = { key: 'settings', label: 'Settings', icon: '⚙' }

export function Sidebar({ activeTab, onTabChange }: { activeTab: string; onTabChange: (tab: string) => void }) {
  const user = useAuthStore((s) => s.user)
  const isProvider = Number(user?.role) === 1

  return (
    <aside className="flex h-full w-56 flex-col border-r border-border bg-sidebar">
      {/* Logo + Home link */}
      <div className="flex items-center gap-2 border-b border-border px-4 py-3">
        <a href="/" className="flex items-center gap-2 text-sm font-semibold text-sidebar-foreground">
          <div className="flex h-6 w-6 items-center justify-center rounded-md bg-primary text-[10px] font-bold text-primary-foreground">
            AI
          </div>
          <span>AI Platform</span>
        </a>
      </div>

      {/* Nav items */}
      <nav className="mt-2 flex-1 space-y-0.5 px-3">
        {BASE_ITEMS.map((item) => (
          <NavBtn key={item.key} item={item} activeTab={activeTab} onClick={onTabChange} />
        ))}

        {/* Provider section */}
        <div className="pt-3 pb-1">
          <div className="flex items-center gap-2 px-2.5 py-1">
            <span className="text-xs font-medium text-sidebar-foreground/40 tracking-wider uppercase">Provider</span>
          </div>
          {isProvider ? (
            PROVIDER_ITEMS.map((item) => (
              <NavBtn key={item.key} item={item} activeTab={activeTab} onClick={onTabChange} />
            ))
          ) : (
            <NavBtn item={{ key: 'provider', label: 'Become Provider', icon: '⊕' }} activeTab={activeTab} onClick={onTabChange} />
          )}
        </div>

        <NavBtn item={SETTINGS_ITEM} activeTab={activeTab} onClick={onTabChange} />
      </nav>
    </aside>
  )
}

function NavBtn({ item, activeTab, onClick }: {
  item: { key: string; label: string; icon: string }
  activeTab: string
  onClick: (k: string) => void
}) {
  return (
    <button
      onClick={() => onClick(item.key)}
      className={`flex w-full items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm transition-colors ${
        activeTab === item.key
          ? 'bg-sidebar-primary text-sidebar-primary-foreground'
          : 'text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-foreground'
      }`}
    >
      <span className="text-sm">{item.icon}</span>
      {item.label}
    </button>
  )
}
