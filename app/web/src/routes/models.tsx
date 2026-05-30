import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useMemo, useState } from 'react'
import { Navbar } from '../components/Navbar'
import { Footer } from '../components/Footer'

export const Route = createFileRoute('/models')({
  component: ModelsPage,
})

interface ModelInfo {
  id: number
  model_code: string
  model_name: string
  developer_name: string
  display_name: string
  model_family: string
  context_window: number
  capabilities: string
  status: string | number
}

const CONTEXT_RANGES = [
  { label: 'All', value: '' },
  { label: '< 4K', value: '<4K' },
  { label: '4K – 8K', value: '4K-8K' },
  { label: '8K – 32K', value: '8K-32K' },
  { label: '32K – 128K', value: '32K-128K' },
  { label: '128K+', value: '128K+' },
] as const

function toggleListItem<T>(list: T[], item: T): T[] {
  return list.includes(item) ? list.filter((i) => i !== item) : [...list, item]
}

function ModelsPage() {
  const navigate = useNavigate()
  const [models, setModels] = useState<ModelInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [selectedDevelopers, setSelectedDevelopers] = useState<string[]>([])
  const [contextRange, setContextRange] = useState('')
  const [selectedCapabilities, setSelectedCapabilities] = useState<string[]>([])
  const [developers, setDevelopers] = useState<string[]>([])

  useEffect(() => {
    fetch('/api/v1/catalog/models')
      .then((r) => r.json())
      .then((res) => {
        const list = res?.data?.list ?? (Array.isArray(res) ? res : [])
        setModels(list.filter((m: ModelInfo) => String(m.status) === 'active'))
      })
      .catch(() => {})
      .finally(() => setLoading(false))

    // Fetch developer list for the filter sidebar
    fetch('/api/v1/developers')
      .then((r) => r.json())
      .then((res) => {
        const list = res?.data?.list ?? (Array.isArray(res) ? res : [])
        setDevelopers(list.map((d: any) => d.name).filter(Boolean))
      })
      .catch(() => {})
  }, [])

  const openDetail = (m: ModelInfo) => {
    navigate({ to: '/models/$code', params: { code: m.model_code } })
  }

  // Derived filter options from real data
  const allCapabilities = useMemo(() => {
    const set = new Set<string>(['chat', 'completion', 'embedding', 'vision', 'audio', 'video', 'tools', 'streaming'])
    models.forEach((m) => {
      ;(m.capabilities || '').split(',').map((c) => c.trim()).filter(Boolean).forEach((c) => set.add(c))
    })
    return [...set].sort()
  }, [models])

  const activeFilterCount =
    (selectedDevelopers.length > 0 ? 1 : 0) +
    (contextRange ? 1 : 0) +
    (selectedCapabilities.length > 0 ? 1 : 0)

  // Composite filter
  const filtered = models.filter((m) => {
    // Search
    if (search) {
      const q = search.toLowerCase()
      if (
        !m.model_name?.toLowerCase().includes(q) &&
        !m.developer_name?.toLowerCase().includes(q) &&
        !m.model_code?.toLowerCase().includes(q)
      )
        return false
    }
    // Developer filter
    if (selectedDevelopers.length > 0 && !selectedDevelopers.includes(m.developer_name)) return false
    // Context window filter
    if (contextRange) {
      const ctx = m.context_window
      switch (contextRange) {
        case '<4K':
          if (ctx >= 4000) return false
          break
        case '4K-8K':
          if (ctx < 4000 || ctx > 8192) return false
          break
        case '8K-32K':
          if (ctx < 8192 || ctx > 32768) return false
          break
        case '32K-128K':
          if (ctx < 32768 || ctx > 131072) return false
          break
        case '128K+':
          if (ctx <= 131072) return false
          break
      }
    }
    // Capabilities filter (OR within selected — match any)
    if (selectedCapabilities.length > 0) {
      const modelCaps = (m.capabilities || '').split(',').map((c) => c.trim())
      const matches = selectedCapabilities.some((c) => modelCaps.includes(c))
      if (!matches) return false
    }
    return true
  })

  return (
    <div className="min-h-screen bg-background">
      <Navbar />

      <div className="mx-auto max-w-7xl px-4 py-12">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-3xl font-bold tracking-tight">Model Factory</h1>
          <p className="mt-1 text-muted-foreground">Browse and compare available LLM models.</p>
        </div>

        {/* Top Search Bar */}
        <div className="mb-6">
          <div className="relative">
            <svg className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" /></svg>
            <input
              type="text"
              placeholder="Search by model name, code, or developer..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="h-11 w-full rounded-xl border border-input bg-card pl-10 pr-4 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            />
            {search && (
              <button onClick={() => setSearch('')} className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground text-sm">✕</button>
            )}
          </div>
        </div>

        {/* Body: Sidebar + Grid */}
        <div className="flex gap-8">
          {/* Left Sidebar Filters */}
          <aside className="hidden w-56 shrink-0 md:block">
            <div className="sticky top-24 space-y-6">
              {/* Active filter badge */}
              {activeFilterCount > 0 && (
                <button onClick={() => { setSelectedDevelopers([]); setContextRange(''); setSelectedCapabilities([]); setSearch('') }} className="text-xs text-primary hover:underline">
                  Clear all {activeFilterCount} filter{activeFilterCount > 1 ? 's' : ''} →
                </button>
              )}

              {/* Developer filter */}
              <fieldset>
                <legend className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">Developer</legend>
                <div className="space-y-1 max-h-48 overflow-y-auto scrollbar-thin">
                  {developers.map((dev) => (
                    <label key={dev} className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1 text-sm transition-colors hover:bg-muted">
                      <input
                        type="checkbox"
                        checked={selectedDevelopers.includes(dev)}
                        onChange={() => setSelectedDevelopers(toggleListItem(selectedDevelopers, dev))}
                        className="h-4 w-4 rounded border-input accent-primary"
                      />
                      <span className="truncate">{dev}</span>
                    </label>
                  ))}
                </div>
              </fieldset>

              {/* Context Window filter */}
              <fieldset>
                <legend className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">Context Window</legend>
                <div className="space-y-1">
                  {CONTEXT_RANGES.map((r) => (
                    <label key={r.value} className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1 text-sm transition-colors hover:bg-muted">
                      <input
                        type="radio"
                        name="contextRange"
                        checked={contextRange === r.value}
                        onChange={() => setContextRange(r.value)}
                        className="h-4 w-4 accent-primary"
                      />
                      <span>{r.label}</span>
                    </label>
                  ))}
                </div>
              </fieldset>

              {/* Capabilities filter */}
              <fieldset>
                <legend className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">Capabilities</legend>
                <div className="space-y-1 max-h-48 overflow-y-auto scrollbar-thin">
                  {allCapabilities.map((cap) => (
                    <label key={cap} className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1 text-sm transition-colors hover:bg-muted">
                      <input
                        type="checkbox"
                        checked={selectedCapabilities.includes(cap)}
                        onChange={() => setSelectedCapabilities(toggleListItem(selectedCapabilities, cap))}
                        className="h-4 w-4 rounded border-input accent-primary"
                      />
                      <span className="capitalize">{cap}</span>
                    </label>
                  ))}
                </div>
              </fieldset>
            </div>
          </aside>

          {/* Model Grid */}
          <div className="flex-1 min-w-0">
            {/* Result count */}
            {!loading && (
              <p className="mb-4 text-xs text-muted-foreground">
                {filtered.length} model{filtered.length !== 1 ? 's' : ''}
                {activeFilterCount > 0 && ' found'}
              </p>
            )}

            {loading ? (
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {Array.from({ length: 8 }).map((_, i) => (
                  <div key={i} className="h-32 animate-pulse rounded-xl bg-muted" />
                ))}
              </div>
            ) : filtered.length === 0 ? (
              <div className="py-20 text-center text-muted-foreground">
                <p className="text-lg">No models found</p>
                <p className="mt-1 text-sm">Try adjusting your search or filters.</p>
              </div>
            ) : (
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {filtered.map((m) => (
                  <button
                    key={m.id}
                    onClick={() => openDetail(m)}
                    className="group text-left rounded-xl border border-border bg-card p-5 transition-all hover:border-primary/50 hover:bg-card/80"
                  >
                    <p className="text-xs text-muted-foreground">{m.developer_name || 'Unknown'}</p>
                    <p className="mt-1 text-base font-semibold">{m.model_name || m.model_code}</p>
                    <div className="mt-3 flex flex-wrap gap-1">
                      {m.model_family && (
                        <span className="rounded-full bg-primary/10 px-2 py-0.5 text-xs text-primary">{m.model_family}</span>
                      )}
                      {m.context_window > 0 && (
                        <span className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">{(m.context_window / 1000).toFixed(0)}K ctx</span>
                      )}
                      {m.capabilities && m.capabilities.split(',').slice(0, 3).map((cap) => (
                        <span key={cap.trim()} className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground capitalize">{cap.trim()}</span>
                      ))}
                    </div>
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>

      <Footer />
    </div>
  )
}
