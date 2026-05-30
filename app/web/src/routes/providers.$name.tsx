import { createFileRoute, Link } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { Navbar } from '../components/Navbar'
import { Footer } from '../components/Footer'

export const Route = createFileRoute('/providers/$name')({
  component: ProviderDetailPage,
})

interface SupplierInfo {
  id: number
  username: string
  email: string
}

interface SupplierStats {
  model_count: number
  total_calls: number
  total_tokens: number
  total_credits: number
}

interface ModelRow {
  model_code: string
  model_name: string
  channel_name: string
  input_price_per_1k: number
  output_price_per_1k: number
  cache_hit_price_per_1k: number
}

function formatUnits(v: number): string {
  const n = Math.abs(v)
  if (n >= 1_000_000_000) return (v / 1_000_000_000).toFixed(1) + 'B'
  if (n >= 1_000_000) return (v / 1_000_000).toFixed(1) + 'M'
  if (n >= 10_000) return (v / 1_000).toFixed(1) + 'K'
  if (n === 0) return '0'
  return v.toLocaleString()
}

function priceText(p: number): string {
  if (p === 0) return '—'
  return '¥' + formatUnits(p)
}

function ProviderDetailPage() {
  const { name } = Route.useParams()
  const [supplier, setSupplier] = useState<SupplierInfo | null>(null)
  const [stats, setStats] = useState<SupplierStats | null>(null)
  const [models, setModels] = useState<ModelRow[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    fetch(`/api/v1/catalog/suppliers/${encodeURIComponent(name)}`)
      .then((r) => r.json())
      .then((res) => {
        if (res.code !== 0) { setError(res.message || 'Supplier not found'); return }
        setSupplier(res.data.supplier)
        setStats(res.data.stats)
        setModels(res.data.models || [])
      })
      .catch(() => setError('Failed to load'))
      .finally(() => setLoading(false))
  }, [name])

  if (loading) {
    return <div className="min-h-screen bg-background"><Navbar /><div className="mx-auto max-w-4xl px-4 py-20 text-center text-muted-foreground">Loading...</div></div>
  }

  if (error || !supplier) {
    return <div className="min-h-screen bg-background"><Navbar /><div className="mx-auto max-w-4xl px-4 py-20 text-center"><p className="text-lg text-muted-foreground">{error || 'Not found'}</p><Link to="/providers" className="mt-4 inline-block text-sm text-primary hover:underline">← Back to Providers</Link></div></div>
  }

  return (
    <div className="min-h-screen bg-background">
      <Navbar />
      <div className="mx-auto max-w-4xl px-4 py-12">
        <div className="mb-8">
          <Link to="/providers" className="text-sm text-muted-foreground hover:text-foreground transition-colors">← Providers</Link>
          <span className="mx-2 text-muted-foreground">/</span>
          <span className="text-sm text-foreground font-medium">{supplier.username}</span>
        </div>

        {/* Supplier Header */}
        <div className="mb-8 rounded-xl border border-border bg-card p-6">
          <div className="flex items-start gap-4">
            <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-xl font-bold text-primary">
              {supplier.username.charAt(0).toUpperCase()}
            </div>
            <div>
              <h1 className="text-2xl font-bold">{supplier.username}</h1>
              <p className="text-sm text-muted-foreground mt-1">{supplier.email}</p>
            </div>
          </div>
        </div>

        {/* Stats */}
        {stats && (
          <div className="mb-8 grid gap-4 sm:grid-cols-4">
            {[
              { label: 'Models', value: stats.model_count.toLocaleString() },
              { label: 'Total Calls', value: formatUnits(stats.total_calls) },
              { label: 'Total Tokens', value: formatUnits(stats.total_tokens) },
              { label: 'Total Credits', value: formatUnits(stats.total_credits) },
            ].map((s) => (
              <div key={s.label} className="rounded-xl border border-border bg-card p-4 text-center">
                <p className="text-2xl font-bold tabular-nums">{s.value}</p>
                <p className="text-xs text-muted-foreground mt-1">{s.label}</p>
              </div>
            ))}
          </div>
        )}

        {/* Models */}
        <div>
          <h2 className="mb-4 text-lg font-semibold tracking-tight">Models ({models.length})</h2>
          {models.length === 0 ? (
            <p className="text-sm text-muted-foreground">No models available.</p>
          ) : (
            <div className="space-y-3">
              {models.map((m, i) => (
                <div key={i} className="rounded-xl border border-border bg-card p-5">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <Link to="/models/$code" params={{ code: m.model_code }} className="text-base font-semibold hover:text-primary transition-colors">
                        {m.model_name}
                      </Link>
                      <p className="text-xs text-muted-foreground mt-1">{m.channel_name}</p>
                    </div>
                    <div className="flex gap-4 text-sm">
                      <div className="text-right">
                        <p className="text-xs text-muted-foreground">Input</p>
                        <p className="font-mono tabular-nums">{priceText(m.input_price_per_1k)}</p>
                      </div>
                      <div className="text-right">
                        <p className="text-xs text-muted-foreground">Output</p>
                        <p className="font-mono tabular-nums">{priceText(m.output_price_per_1k)}</p>
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
      <Footer />
    </div>
  )
}
