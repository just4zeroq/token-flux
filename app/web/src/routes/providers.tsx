import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { Navbar } from '../components/Navbar'
import { Footer } from '../components/Footer'

export const Route = createFileRoute('/providers')({
  component: ProvidersPage,
})

interface SupplierRow {
  id: number
  username: string
  email: string
  model_count: number
  total_calls: number
  total_tokens: number
  total_credits: number
}

const SORT_OPTIONS = [
  { label: 'Model Count', value: 'model_count' },
  { label: 'Call Volume', value: 'calls' },
  { label: 'Total Credits', value: 'credits' },
]

function formatUnits(v: number): string {
  const n = Math.abs(v)
  if (n >= 1_000_000_000) return (v / 1_000_000_000).toFixed(1) + 'B'
  if (n >= 1_000_000) return (v / 1_000_000).toFixed(1) + 'M'
  if (n >= 10_000) return (v / 1_000).toFixed(1) + 'K'
  if (n === 0) return '0'
  return v.toLocaleString()
}

function ProvidersPage() {
  const navigate = useNavigate()
  const [suppliers, setSuppliers] = useState<SupplierRow[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [sort, setSort] = useState('model_count')

  const fetchSuppliers = () => {
    setLoading(true)
    fetch(`/api/v1/catalog/suppliers?sort=${sort}&search=${encodeURIComponent(search)}`)
      .then((r) => r.json())
      .then((res) => setSuppliers(res?.data?.list ?? []))
      .catch(() => {})
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchSuppliers() }, [sort])

  return (
    <div className="min-h-screen bg-background">
      <Navbar />

      <div className="border-b border-border bg-muted/30">
        <div className="mx-auto max-w-7xl px-4 py-12">
          <div className="flex items-end justify-between">
            <div>
              <h1 className="text-2xl font-bold tracking-tight">Providers</h1>
              <p className="mt-1 text-sm text-muted-foreground">
                {loading ? 'Loading...' : `${suppliers.length} providers on the platform`}
              </p>
            </div>
            <a
              href="/providers-apply"
              className="inline-flex h-9 items-center rounded-lg bg-primary px-4 text-xs font-medium text-primary-foreground transition-opacity hover:opacity-90"
            >
              Apply as Provider
            </a>
          </div>

          <div className="mt-4 flex gap-3">
            <div className="flex flex-1 max-w-sm">
              <input
                type="text"
                placeholder="Search providers..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && fetchSuppliers()}
                className="h-9 w-full rounded-lg border border-input bg-card px-3 text-sm focus:outline-none"
              />
              <button onClick={fetchSuppliers} className="ml-2 h-9 rounded-lg border border-border bg-card px-3 text-sm">
                Search
              </button>
            </div>
            <select
              value={sort}
              onChange={(e) => setSort(e.target.value)}
              className="h-9 rounded-lg border border-input bg-card px-3 text-sm"
            >
              {SORT_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>{o.label}</option>
              ))}
            </select>
          </div>
        </div>
      </div>

      <div className="mx-auto max-w-7xl px-4 py-8">
        {loading ? (
          <div className="grid gap-4 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
            {Array.from({ length: 8 }).map((_, i) => (
              <div key={i} className="h-36 animate-pulse rounded-xl bg-muted" />
            ))}
          </div>
        ) : suppliers.length === 0 ? (
          <div className="py-20 text-center text-muted-foreground">
            <p className="text-lg">No providers found</p>
            <p className="mt-1 text-sm">Try adjusting your search.</p>
          </div>
        ) : (
          <div className="grid gap-4 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
            {suppliers.map((s) => (
              <button
                key={s.id}
                onClick={() => navigate({ to: '/providers/$name', params: { name: s.username } })}
                className="text-left rounded-xl border border-border bg-card p-5 transition-all hover:border-primary/50"
              >
                <div className="flex items-center gap-3 mb-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10 text-sm font-bold text-primary">
                    {s.username.charAt(0).toUpperCase()}
                  </div>
                  <div>
                    <p className="font-semibold text-sm">{s.username}</p>
                    <p className="text-xs text-muted-foreground truncate max-w-[160px]">{s.email}</p>
                  </div>
                </div>
                <div className="space-y-1 text-xs text-muted-foreground">
                  <div className="flex justify-between">
                    <span>Models</span>
                    <span className="font-mono text-foreground">{s.model_count}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Calls</span>
                    <span className="font-mono text-foreground">{formatUnits(s.total_calls)}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Tokens</span>
                    <span className="font-mono text-foreground">{formatUnits(s.total_tokens)}</span>
                  </div>
                </div>
              </button>
            ))}
          </div>
        )}
      </div>

      <Footer />
    </div>
  )
}
