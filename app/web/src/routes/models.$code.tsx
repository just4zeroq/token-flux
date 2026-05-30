import { createFileRoute, Link } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { Navbar } from '../components/Navbar'
import { Footer } from '../components/Footer'

export const Route = createFileRoute('/models/$code')({
  component: ModelDetailPage,
})

interface ModelInfo {
  id: number
  developer_name: string
  model_name: string
  model_code: string
  display_name: string
  model_family: string
  description: string
  capabilities: string
  context_window: number
  status: string
}

interface ProviderInfo {
  channel_id: number
  channel_name: string
  channel_code: string
  upstream_model_name: string
  cache_hit_price_per_1k: number
  cache_miss_price_per_1k: number
  output_price_per_1k: number
}

function priceText(p: number): string {
  if (p === 0) return '—'
  return '¥' + (p / 1_000_000).toFixed(4)
}

function formatContext(ctx: number): string {
  if (ctx >= 1_000_000) return (ctx / 1_000_000).toFixed(1) + 'M'
  if (ctx >= 1_000) return (ctx / 1_000).toFixed(0) + 'K'
  return String(ctx)
}

function ModelDetailPage() {
  const { code } = Route.useParams()
  const [model, setModel] = useState<ModelInfo | null>(null)
  const [providers, setProviders] = useState<ProviderInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    fetch(`/api/v1/catalog/models/${code}`)
      .then((r) => r.json())
      .then((res) => {
        if (res.code !== 0) { setError(res.message || 'Model not found'); return }
        setModel(res.data.model)
        setProviders(res.data.providers || [])
      })
      .catch(() => setError('Failed to load model'))
      .finally(() => setLoading(false))
  }, [code])

  if (loading) {
    return (
      <div className="min-h-screen bg-background">
        <Navbar />
        <div className="mx-auto max-w-4xl px-4 py-20 text-center text-muted-foreground">Loading...</div>
      </div>
    )
  }

  if (error || !model) {
    return (
      <div className="min-h-screen bg-background">
        <Navbar />
        <div className="mx-auto max-w-4xl px-4 py-20 text-center">
          <p className="text-lg text-muted-foreground">{error || 'Model not found'}</p>
          <Link to="/models" className="mt-4 inline-block text-sm text-primary hover:underline">← Back to Models</Link>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-background">
      <Navbar />

      <div className="mx-auto max-w-4xl px-4 py-12">
        {/* Breadcrumb */}
        <div className="mb-8">
          <Link to="/models" className="text-sm text-muted-foreground hover:text-foreground transition-colors">
            ← Models
          </Link>
          <span className="mx-2 text-muted-foreground">/</span>
          <span className="text-sm text-foreground font-medium">{model.model_code}</span>
        </div>

        {/* Model Header */}
        <div className="flex flex-wrap items-start justify-between gap-4 mb-8">
          <div>
            <h1 className="text-3xl font-bold tracking-tight">{model.display_name || model.model_name}</h1>
            <div className="mt-2 flex flex-wrap items-center gap-2 text-sm">
              <span className="font-medium text-foreground">{model.developer_name}</span>
              <span className="text-muted-foreground">·</span>
              <span className="text-muted-foreground">{model.model_family}</span>
              <span className="text-muted-foreground">·</span>
              <span className="text-muted-foreground">{formatContext(model.context_window)} context</span>
            </div>
            {model.capabilities && (
              <div className="mt-3 flex flex-wrap gap-1.5">
                {model.capabilities.split(',').map((c) => (
                  <span key={c.trim()} className="rounded-full border border-primary/30 bg-primary/5 px-2.5 py-0.5 text-xs text-primary capitalize">{c.trim()}</span>
                ))}
              </div>
            )}
          </div>
          <a
            href="/console"
            className="inline-flex h-10 shrink-0 items-center rounded-lg bg-primary px-5 text-sm font-medium text-primary-foreground transition-opacity hover:opacity-90"
          >
            Try in Console
          </a>
        </div>

        {/* Description */}
        {model.description && (
          <div className="mb-8 rounded-xl border border-border bg-card p-5">
            <p className="text-sm text-muted-foreground leading-relaxed">{model.description}</p>
          </div>
        )}

        {/* Providers */}
        <div>
          <h2 className="mb-4 text-lg font-semibold tracking-tight">
            Available Providers
            <span className="ml-2 text-sm font-normal text-muted-foreground">({providers.length})</span>
          </h2>
          {providers.length === 0 ? (
            <p className="text-sm text-muted-foreground">No providers available for this model yet.</p>
          ) : (
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {providers.map((p) => (
                <div key={p.channel_id} className="rounded-xl border border-border bg-card p-5 transition-all hover:border-primary/30">
                  <p className="font-semibold text-sm">{p.channel_name}</p>
                  {p.upstream_model_name && (
                    <p className="text-xs text-muted-foreground mt-0.5">{p.upstream_model_name}</p>
                  )}
                  <div className="mt-4 space-y-1.5 text-sm">
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Input /1K</span>
                      <span className="font-mono tabular-nums">{priceText(p.cache_miss_price_per_1k)}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Output /1K</span>
                      <span className="font-mono tabular-nums">{priceText(p.output_price_per_1k)}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Cache hit /1K</span>
                      <span className="font-mono tabular-nums">{priceText(p.cache_hit_price_per_1k)}</span>
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
