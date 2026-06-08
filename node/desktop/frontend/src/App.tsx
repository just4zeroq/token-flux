import { useState } from 'react'
import { BrowserRouter, Routes, Route, NavLink, Navigate } from 'react-router-dom'
import Dashboard from './pages/Dashboard'
import Custom from './pages/Custom'
import Keys from './pages/Keys'
import Usage from './pages/Usage'
import Network from './pages/Network'
import Settings from './pages/Settings'
import AuthWidget from './components/AuthWidget'
import { useToast } from './stores/nodeStore'

interface NavItem { path: string; label: string; icon: string }
interface NavGroup { label: string; icon: string; items: NavItem[] }

const GROUPS: NavGroup[] = [
  { label: 'Workspace', icon: '⊞', items: [] },
  { label: 'Agents', icon: '●', items: [] },
  { label: 'Markets', icon: '📊', items: [] },
  {
    label: 'Tokens', icon: '◆',
    items: [
      { path: '/dashboard', label: 'Dashboard', icon: '◉' },
      { path: '/keys', label: 'Keys', icon: '🔑' },
      { path: '/network', label: 'Network', icon: '⚡' },
      { path: '/custom', label: 'Custom', icon: '⚙' },
      { path: '/usage', label: 'Usage', icon: '▤' },
      { path: '/settings', label: 'Settings', icon: '⚙' },
    ],
  },
  { label: 'Skills', icon: '⚡', items: [] },
  { label: 'Resources', icon: '📦', items: [] },
]

function ToastContainer() {
  const { toasts, removeToast } = useToast()
  if (toasts.length === 0) return null
  return (
    <div className="toast-container">
      {toasts.map((t) => (
        <div key={t.id} className={`toast toast-${t.type}`} onClick={() => removeToast(t.id)}>
          {t.message}
        </div>
      ))}
    </div>
  )
}

function AppLayout() {
  const [col1Open, setCol1Open] = useState(true)
  const [col2Open, setCol2Open] = useState(true)
  const [activeGroup, setActiveGroup] = useState('Tokens')

  const currentGroup = GROUPS.find((g) => g.label === activeGroup)
  const hasCol2 = currentGroup && currentGroup.items.length > 0

  const handleCol1Click = (g: NavGroup) => {
    setActiveGroup(g.label)
    if (g.label === 'Settings') return
    if (!col2Open && g.items.length > 0) setCol2Open(true)
  }

  return (
    <div className="app-shell">
      {/* Top bar */}
      <div className="topbar">
        <div className="topbar-brand">
          <div className="topbar-logo">T</div>
          <span className="topbar-title">Token Flux Node</span>
          <span className="topbar-version">v0.1</span>
        </div>
        <div className="topbar-right"><AuthWidget /></div>
      </div>

      <div className="app-body">
        {/* Column 1 — Level-1 menu */}
        <aside className={`col1${col1Open ? '' : ' collapsed'}`}>
          <div className="col-header">
            {col1Open && <span className="col-title">Menu</span>}
            <button className="col-toggle" onClick={() => setCol1Open(!col1Open)} title={col1Open ? 'Collapse' : 'Expand'}>
              {col1Open ? '◀' : '▶'}
            </button>
          </div>
          <nav className="col1-nav">
            {GROUPS.map((g) => (
              <div
                key={g.label}
                className={`col1-item${activeGroup === g.label ? ' active' : ''}`}
                onClick={() => handleCol1Click(g)}
              >
                <span className="col1-icon">{g.icon}</span>
                {col1Open && <span className="col1-label">{g.label}</span>}
              </div>
            ))}
            {/* Settings — always bottom */}
            <NavLink
              to="/settings"
              className={({ isActive }) => `col1-item${isActive ? ' active' : ''}`}
              onClick={() => { setActiveGroup('Settings'); setCol2Open(false) }}
            >
              <span className="col1-icon">⚙</span>
              {col1Open && <span className="col1-label">Settings</span>}
            </NavLink>
          </nav>
        </aside>

        {/* Column 2 — Level-2 sub-items */}
        {col2Open && hasCol2 && (
          <aside className="col2">
            <div className="col-header">
              <span className="col-title">{activeGroup}</span>
              <button className="col-toggle" onClick={() => setCol2Open(false)} title="Collapse">◀</button>
            </div>
            <nav className="col2-nav">
              {currentGroup!.items.map((item) => (
                <NavLink key={item.path} to={item.path} className={({ isActive }) => isActive ? 'active' : ''}>
                  <span className="nav-icon">{item.icon}</span>
                  <span className="nav-label">{item.label}</span>
                </NavLink>
              ))}
            </nav>
          </aside>
        )}

        {/* Column 2 collapsed — thin re-expand bar */}
        {!col2Open && hasCol2 && (
          <div className="col2-collapsed" onClick={() => setCol2Open(true)} title="Expand sub-menu">▶</div>
        )}

        {/* Main Content */}
        <main className="main-content">
          <Routes>
            <Route path="/" element={<Navigate to="/dashboard" replace />} />
            <Route path="/dashboard" element={<Dashboard />} />
            <Route path="/keys" element={<Keys />} />
            <Route path="/custom" element={<Custom />} />
            <Route path="/usage" element={<Usage />} />
            <Route path="/network" element={<Network />} />
            <Route path="/settings" element={<Settings />} />
          </Routes>
        </main>
      </div>

      <ToastContainer />
    </div>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <AppLayout />
    </BrowserRouter>
  )
}
