import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { Navbar } from '../components/Navbar'
import { Footer } from '../components/Footer'

export const Route = createFileRoute('/')({
  component: LandingPage,
})

const CAPABILITIES = [
  { title: 'LLM API Keys', desc: 'Host & manage API keys for 50+ LLM providers with granular access control.', icon: '⚷' },
  { title: 'MCP Gateway', desc: 'Standardized Model Context Protocol gateway for seamless provider integration.', icon: '⇆' },
  { title: 'Agent Runtime', desc: 'Deploy & scale autonomous agents with built-in monitoring & logging.', icon: '◉' },
  { title: 'Double-Entry Ledger', desc: 'Immutable settlement system with automatic reconciliation & audit trails.', icon: '☰' },
  { title: 'Alipay & WeChat Pay', desc: 'Integrated Chinese payment gateways for frictionless credit recharges.', icon: '⟳' },
  { title: 'Developer Console', desc: 'Real-time dashboard for usage metrics, key rotation & team management.', icon: '▤' },
]

const FEATURED_MODELS = [
  { name: 'GPT-4o', provider: 'OpenAI', price: '$2.50/1M input', color: 'border-l-green-500' },
  { name: 'Claude 4', provider: 'Anthropic', price: '$3.00/1M input', color: 'border-l-orange-500' },
  { name: 'DeepSeek-V3', provider: 'DeepSeek', price: '$0.50/1M input', color: 'border-l-blue-500' },
  { name: 'Gemini 2.5', provider: 'Google', price: '$1.25/1M input', color: 'border-l-purple-500' },
]

function LandingPage() {
  const [models, setModels] = useState(FEATURED_MODELS)

  useEffect(() => {
    fetch('/api/v1/provider/llm/models')
      .then((r) => r.json())
      .then((data) => {
        if (Array.isArray(data) && data.length > 0) {
          setModels(
            data.slice(0, 4).map((m: any) => ({
              name: m.model_name || m.model_code || 'Unknown',
              provider: m.developer_name || 'Unknown',
              price: m.price_input_per_1k ? `¥${m.price_input_per_1k}/1K` : '—',
              color: 'border-l-primary',
            }))
          )
        }
      })
      .catch(() => {})
  }, [])

  return (
    <div className="min-h-screen bg-background">
      <Navbar />

      {/* Hero */}
      <section className="relative overflow-hidden border-b border-border">
        <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_top,_var(--tw-gradient-stops))] from-primary/10 via-transparent to-transparent" />
        <div className="relative mx-auto max-w-7xl px-4 py-24 md:py-32">
          <div className="grid items-center gap-12 md:grid-cols-2">
            <div className="animate-fade-in">
              <h1 className="text-4xl font-bold leading-tight tracking-tight md:text-6xl">
                Build the Next Generation of{' '}
                <span className="text-primary">AI Applications</span>
              </h1>
              <p className="mt-4 text-lg text-muted-foreground md:text-xl">
                API key hosting, model gateway, MCP protocol, and double-entry settlement —
                everything you need to ship LLM-powered products.
              </p>
              <div className="mt-8 flex flex-wrap gap-4">
                <a href="/register" className="inline-flex h-12 items-center justify-center rounded-xl bg-primary px-6 text-sm font-semibold text-primary-foreground transition-all hover:opacity-90">
                  Get Started Free
                </a>
                <a href="/models" className="inline-flex h-12 items-center justify-center rounded-xl border border-border bg-card px-6 text-sm font-semibold text-foreground transition-colors hover:bg-muted">
                  View Models →
                </a>
              </div>
              <div className="mt-12 flex gap-8 text-sm text-muted-foreground">
                <div><span className="text-2xl font-bold text-foreground">50+</span><br />Models</div>
                <div><span className="text-2xl font-bold text-foreground">99.9%</span><br />Uptime</div>
                <div><span className="text-2xl font-bold text-foreground">10K+</span><br />Developers</div>
              </div>
            </div>
            {/* Hero Bento */}
            <div className="hidden md:grid grid-cols-2 gap-3">
              <div className="col-span-2 rounded-2xl border border-border bg-card p-6">
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <span className="h-3 w-3 rounded-full bg-primary" />
                  API Status — All Systems Operational
                </div>
              </div>
              {[
                { label: 'Avg Latency', value: '< 200ms' },
                { label: 'Available Models', value: '50+' },
                { label: 'Starting Credits', value: 'Free' },
                { label: 'Support', value: '24/7' },
              ].map((item) => (
                <div key={item.label} className="rounded-2xl border border-border bg-card p-5">
                  <p className="text-xs text-muted-foreground">{item.label}</p>
                  <p className="mt-1 text-lg font-bold">{item.value}</p>
                </div>
              ))}
            </div>
          </div>
        </div>
      </section>

      {/* Capabilities Bento Grid */}
      <section className="mx-auto max-w-7xl px-4 py-20">
        <div className="mb-12 text-center">
          <h2 className="text-3xl font-bold tracking-tight">Everything You Need</h2>
          <p className="mt-2 text-muted-foreground">One platform. All the tools to build with LLMs.</p>
        </div>
        <div className="grid gap-4 md:grid-cols-3">
          {CAPABILITIES.map((cap) => (
            <div key={cap.title} className="group rounded-2xl border border-border bg-card p-6 transition-all hover:border-primary/50 hover:bg-card/80">
              <span className="text-2xl">{cap.icon}</span>
              <h3 className="mt-3 font-semibold">{cap.title}</h3>
              <p className="mt-1 text-sm text-muted-foreground">{cap.desc}</p>
            </div>
          ))}
        </div>
      </section>

      {/* Model Showcase */}
      <section className="border-y border-border bg-muted/30">
        <div className="mx-auto max-w-7xl px-4 py-20">
          <div className="mb-12 flex items-end justify-between">
            <div>
              <h2 className="text-3xl font-bold tracking-tight">Featured Models</h2>
              <p className="mt-2 text-muted-foreground">Browse popular models to get started.</p>
            </div>
            <a href="/models" className="hidden text-sm font-medium text-primary hover:underline md:block">View All →</a>
          </div>
          <div className="grid gap-4 md:grid-cols-4">
            {models.map((m) => (
              <div key={m.name} className={`rounded-xl border border-border bg-card p-5 border-l-4 transition-all hover:border-border hover:bg-card/80 ${m.color}`}>
                <p className="text-sm text-muted-foreground">{m.provider}</p>
                <p className="mt-1 text-lg font-semibold">{m.name}</p>
                <p className="mt-2 text-xs text-muted-foreground">{m.price}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="mx-auto max-w-7xl px-4 py-20 text-center">
        <div className="mx-auto max-w-2xl">
          <h2 className="text-3xl font-bold tracking-tight">Ready to Ship?</h2>
          <p className="mt-3 text-lg text-muted-foreground">Get 100 free credits on signup. No credit card required.</p>
          <a href="/register" className="mt-8 inline-flex h-12 items-center justify-center rounded-xl bg-primary px-8 text-sm font-semibold text-primary-foreground transition-all hover:opacity-90">
            Create Free Account
          </a>
          <p className="mt-4 text-xs text-muted-foreground">Free tier includes 100 credits. No commitment. Cancel anytime.</p>
        </div>
      </section>

      <Footer />
    </div>
  )
}
