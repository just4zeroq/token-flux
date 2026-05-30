# User-Facing Pages Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build landing page, model factory, providers page, and console with tab navigation for ai-platform React SPA.

**Architecture:** TanStack Router flat routes restructured to nested layouts. Console uses parent route with Outlet + sidebar for tab navigation. Public pages share Navbar/Footer components. shadcn/ui for consistent design.

**Tech Stack:** React 19, TanStack Router v1, TanStack Query, Tailwind CSS v4, shadcn/ui (radix-nova), lucide-react, zustand

---

### Task 1: Add shadcn UI components

**Files:**
- Create: `app/web/src/components/ui/button.tsx`
- Create: `app/web/src/components/ui/card.tsx`
- Create: `app/web/src/components/ui/badge.tsx`
- Create: `app/web/src/components/ui/dialog.tsx`
- Create: `app/web/src/components/ui/input.tsx`
- Create: `app/web/src/components/ui/textarea.tsx`
- Create: `app/web/src/components/ui/label.tsx`
- Create: `app/web/src/components/ui/separator.tsx`
- Create: `app/web/src/components/ui/skeleton.tsx`

- [ ] **Step 1: Add shadcn button component**

Read shadcn button source from node_modules or create manually. Button supports variants (default/destructive/outline/secondary/ghost/link) and sizes (default/sm/lg/icon).

```tsx
// src/components/ui/button.tsx
import { Slot } from "@radix-ui/react-slot"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "@/lib/utils"

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground shadow hover:bg-primary/90",
        destructive: "bg-destructive text-destructive-foreground shadow-sm hover:bg-destructive/90",
        outline: "border border-input bg-background shadow-sm hover:bg-accent hover:text-accent-foreground",
        secondary: "bg-secondary text-secondary-foreground shadow-sm hover:bg-secondary/80",
        ghost: "hover:bg-accent hover:text-accent-foreground",
        link: "text-primary underline-offset-4 hover:underline",
      },
      size: {
        default: "h-9 px-4 py-2",
        sm: "h-8 rounded-md px-3 text-xs",
        lg: "h-10 rounded-md px-8",
        icon: "h-9 w-9",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> {
  asChild?: boolean
}

function Button({ className, variant, size, asChild = false, ...props }: ButtonProps) {
  const Comp = asChild ? Slot : "button"
  return <Comp className={cn(buttonVariants({ variant, size, className }))} {...props} />
}
export { Button, buttonVariants }
```

- [ ] **Step 2: Add shadcn card component**

```tsx
// src/components/ui/card.tsx
import { cn } from "@/lib/utils"

function Card({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("rounded-xl border bg-card text-card-foreground shadow", className)} {...props} />
}
function CardHeader({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("flex flex-col space-y-1.5 p-6", className)} {...props} />
}
function CardTitle({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("font-semibold leading-none tracking-tight", className)} {...props} />
}
function CardDescription({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("text-sm text-muted-foreground", className)} {...props} />
}
function CardContent({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("p-6 pt-0", className)} {...props} />
}
function CardFooter({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("flex items-center p-6 pt-0", className)} {...props} />
}
export { Card, CardHeader, CardFooter, CardTitle, CardDescription, CardContent }
```

- [ ] **Step 3: Add remaining shadcn components**

Create `badge.tsx`, `dialog.tsx`, `input.tsx`, `textarea.tsx`, `label.tsx`, `separator.tsx`, `skeleton.tsx`.

For each, use standard shadcn/ui source adapted for the radix-nova style (already configured in components.json). All use `cn()` from `@/lib/utils` and follow the same pattern as Button/Card above.

Detailed contents:
- **Badge**: variants (default/secondary/destructive/outline), uses `cn` + cva
- **Dialog**: uses `@radix-ui/react-dialog` with overlay, content, header, title, description, footer
- **Input**: styled input element with `flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm`
- **Textarea**: similar to Input but textarea element
- **Label**: uses `@radix-ui/react-label` with peer-disabled styles
- **Separator**: uses `@radix-ui/react-separator`
- **Skeleton**: animated pulse div

Reference existing `button.tsx` pattern for cva usage and `cn()` imports.

- [ ] **Step 4: Verify**

```bash
cd /d/code/github/ai-platform/app/web && npx tsc --noEmit 2>&1 | head -20
```

Expected: TypeScript passes (or minimal errors unrelated to these files).

- [ ] **Step 5: Commit**

```bash
git add app/web/src/components/ui/
git commit -m "feat: add shadcn UI components (button, card, badge, dialog, input, textarea, label, separator, skeleton)"
```

---

### Task 2: Create shared Navbar + Footer components

**Files:**
- Create: `app/web/src/components/navbar.tsx`
- Create: `app/web/src/components/footer.tsx`

- [ ] **Step 1: Create Navbar component**

Responsive nav header with:
- Logo/brand ("AI Platform")
- Nav links: Home (`/`), Models (`/models`), Providers (`/providers`)
- Right side: if logged in → username + "Console" link + Logout; if not → Login / Register buttons
- Mobile hamburger menu (simplified: hide links on small screens, show hamburger toggle)

```tsx
// src/components/navbar.tsx
import { useState } from "react"
import { Link } from "@tanstack/react-router"
import { useAuthStore } from "@/stores/auth"
import { Button } from "@/components/ui/button"
import { Menu, X } from "lucide-react"

const navLinks = [
  { to: "/", label: "Home" },
  { to: "/models", label: "Models" },
  { to: "/providers", label: "Providers" },
]

export function Navbar() {
  const token = useAuthStore((s) => s.token)
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)
  const [mobileOpen, setMobileOpen] = useState(false)

  return (
    <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="container mx-auto flex h-16 items-center justify-between px-4">
        {/* Logo */}
        <Link to="/" className="flex items-center gap-2 font-bold text-lg">
          AI Platform
        </Link>

        {/* Desktop nav */}
        <nav className="hidden md:flex items-center gap-6 text-sm">
          {navLinks.map((link) => (
            <Link key={link.to} to={link.to} className="text-muted-foreground hover:text-foreground transition-colors">
              {link.label}
            </Link>
          ))}
        </nav>

        {/* Desktop auth */}
        <div className="hidden md:flex items-center gap-3">
          {token ? (
            <>
              <span className="text-sm text-muted-foreground">{user?.username}</span>
              <Link to="/console"><Button variant="outline" size="sm">Console</Button></Link>
              <Button variant="ghost" size="sm" onClick={logout}>Logout</Button>
            </>
          ) : (
            <>
              <Link to="/login"><Button variant="ghost" size="sm">Login</Button></Link>
              <Link to="/login"><Button size="sm">Get Started</Button></Link>
            </>
          )}
        </div>

        {/* Mobile menu toggle */}
        <button className="md:hidden" onClick={() => setMobileOpen(!mobileOpen)}>
          {mobileOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
        </button>
      </div>

      {/* Mobile nav */}
      {mobileOpen && (
        <div className="md:hidden border-t px-4 py-4 space-y-3">
          {navLinks.map((link) => (
            <Link key={link.to} to={link.to} onClick={() => setMobileOpen(false)} className="block text-muted-foreground hover:text-foreground">
              {link.label}
            </Link>
          ))}
          <div className="pt-2 border-t">
            {token ? (
              <>
                <p className="text-sm text-muted-foreground mb-2">{user?.username}</p>
                <Link to="/console" onClick={() => setMobileOpen(false)}><Button variant="outline" size="sm" className="w-full">Console</Button></Link>
                <Button variant="ghost" size="sm" className="w-full mt-1" onClick={logout}>Logout</Button>
              </>
            ) : (
              <>
                <Link to="/login" onClick={() => setMobileOpen(false)}><Button variant="outline" size="sm" className="w-full">Login</Button></Link>
              </>
            )}
          </div>
        </div>
      )}
    </header>
  )
}
```

- [ ] **Step 2: Create Footer component**

```tsx
// src/components/footer.tsx
export function Footer() {
  return (
    <footer className="border-t py-8 bg-muted/30">
      <div className="container mx-auto px-4">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
          <div>
            <h3 className="font-semibold mb-3">AI Platform</h3>
            <p className="text-sm text-muted-foreground">Reliable LLM API services for your AI applications.</p>
          </div>
          <div>
            <h3 className="font-semibold mb-3">Platform</h3>
            <ul className="space-y-2 text-sm text-muted-foreground">
              <li><a href="/models" className="hover:text-foreground">Models</a></li>
              <li><a href="/providers" className="hover:text-foreground">Providers</a></li>
            </ul>
          </div>
          <div>
            <h3 className="font-semibold mb-3">Contact</h3>
            <ul className="space-y-2 text-sm text-muted-foreground">
              <li>Telegram: @aiplatform</li>
              <li>Email: support@aiplatform.com</li>
            </ul>
          </div>
        </div>
        <div className="mt-8 pt-4 border-t text-center text-sm text-muted-foreground">
          &copy; {new Date().getFullYear()} AI Platform. All rights reserved.
        </div>
      </div>
    </footer>
  )
}
```

- [ ] **Step 3: Verify TypeScript**

```bash
cd /d/code/github/ai-platform/app/web && npx tsc --noEmit 2>&1 | head -10
```

Expected: no errors or minimal.

- [ ] **Step 4: Commit**

```bash
git add app/web/src/components/navbar.tsx app/web/src/components/footer.tsx
git commit -m "feat: add shared Navbar and Footer components"
```

---

### Task 3: Rewrite Landing Page (/)

**Files:**
- Modify: `app/web/src/routes/index.tsx`

- [ ] **Step 1: Rewrite index.tsx as landing page**

```tsx
// src/routes/index.tsx (full rewrite)
import { createFileRoute, Link } from "@tanstack/react-router"
import { Navbar } from "@/components/navbar"
import { Footer } from "@/components/footer"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Sparkles, Shield, Zap, Code2 } from "lucide-react"

export const Route = createFileRoute("/")({
  component: HomePage,
})

const models = [
  { name: "GPT-4o", provider: "OpenAI", capability: "chat", status: "active" },
  { name: "Claude 3.5 Sonnet", provider: "Anthropic", capability: "chat", status: "active" },
  { name: "Gemini Pro", provider: "Google", capability: "chat", status: "active" },
  { name: "text-embedding-3", provider: "OpenAI", capability: "embedding", status: "active" },
]

const advantages = [
  { icon: Zap, title: "Low Latency", desc: "Optimized routing for fastest response times" },
  { icon: Shield, title: "High Reliability", desc: "99.9% uptime with automatic failover" },
  { icon: Code2, title: "Developer Friendly", desc: "OpenAI-compatible API, drop-in integration" },
  { icon: Sparkles, title: "Competitive Pricing", desc: "Pay only for what you use, no hidden fees" },
]

function HomePage() {
  return (
    <div className="min-h-screen flex flex-col">
      <Navbar />

      <main className="flex-1">
        {/* Hero */}
        <section className="py-20 md:py-32 text-center px-4">
          <h1 className="text-4xl md:text-6xl font-bold tracking-tight max-w-3xl mx-auto">
            Reliable LLM APIs for Your AI Applications
          </h1>
          <p className="mt-6 text-lg text-muted-foreground max-w-2xl mx-auto">
            Access leading language models through a single, OpenAI-compatible API. 
            High availability, transparent pricing, and developer-first tooling.
          </p>
          <div className="mt-8 flex items-center justify-center gap-4">
            <Link to="/login">
              <Button size="lg">Get Started</Button>
            </Link>
            <Link to="/models">
              <Button variant="outline" size="lg">View Models</Button>
            </Link>
          </div>
        </section>

        {/* Model Showcase */}
        <section className="py-16 bg-muted/30 px-4">
          <div className="container mx-auto">
            <h2 className="text-2xl font-bold text-center mb-8">Supported Models</h2>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 max-w-4xl mx-auto">
              {models.map((m) => (
                <Card key={m.name} className="hover:shadow-md transition-shadow">
                  <CardContent className="p-4">
                    <p className="font-semibold text-sm">{m.name}</p>
                    <p className="text-xs text-muted-foreground mt-1">{m.provider}</p>
                    <div className="flex items-center gap-2 mt-3">
                      <Badge variant="secondary" className="text-xs">{m.capability}</Badge>
                      <Badge variant="outline" className="text-xs text-green-600 border-green-200 bg-green-50">Active</Badge>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          </div>
        </section>

        {/* Core Advantages */}
        <section className="py-16 px-4">
          <div className="container mx-auto">
            <h2 className="text-2xl font-bold text-center mb-8">Why AI Platform</h2>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 max-w-5xl mx-auto">
              {advantages.map((a) => (
                <Card key={a.title} className="text-center">
                  <CardContent className="p-6">
                    <a.icon className="h-8 w-8 mx-auto text-primary" />
                    <h3 className="font-semibold mt-4">{a.title}</h3>
                    <p className="text-sm text-muted-foreground mt-2">{a.desc}</p>
                  </CardContent>
                </Card>
              ))}
            </div>
          </div>
        </section>

        {/* Provider CTA */}
        <section className="py-16 bg-muted/30 px-4">
          <div className="container mx-auto text-center">
            <h2 className="text-2xl font-bold">Become a Provider</h2>
            <p className="mt-3 text-muted-foreground max-w-xl mx-auto">
              Offer your models on our platform and reach developers worldwide.
            </p>
            <Link to="/providers">
              <Button className="mt-6" variant="outline" size="lg">Learn More</Button>
            </Link>
          </div>
        </section>
      </main>

      <Footer />
    </div>
  )
}
```

- [ ] **Step 2: Verify**

```bash
cd /d/code/github/ai-platform/app/web && npx tsc --noEmit 2>&1 | head -10
```

Expected: no errors (or only minor unrelated warnings).

- [ ] **Step 3: Commit**

```bash
git add app/web/src/routes/index.tsx
git commit -m "feat: rewrite landing page with hero, model showcase, advantages, provider CTA"
```

---

### Task 4: Rewrite Model Factory (/models)

**Files:**
- Create: `app/web/src/components/model-card.tsx`
- Modify: `app/web/src/routes/models.tsx`

- [ ] **Step 1: Create ModelCard component**

```tsx
// src/components/model-card.tsx
import { Card, CardContent } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"

interface ModelCardProps {
  name: string
  provider: string
  capabilities: string[]
  priceInput: string
  priceOutput: string
  status: string
  onClick?: () => void
}

export function ModelCard({ name, provider, capabilities, priceInput, priceOutput, status, onClick }: ModelCardProps) {
  return (
    <Card className="cursor-pointer hover:shadow-md transition-shadow" onClick={onClick}>
      <CardContent className="p-5">
        <div className="flex items-start justify-between">
          <div>
            <p className="font-semibold">{name}</p>
            <p className="text-xs text-muted-foreground mt-0.5">{provider}</p>
          </div>
          <Badge variant={status === "active" ? "default" : "secondary"} className="text-xs">
            {status}
          </Badge>
        </div>
        <div className="flex flex-wrap gap-1.5 mt-3">
          {capabilities.map((c) => (
            <Badge key={c} variant="secondary" className="text-xs">{c}</Badge>
          ))}
        </div>
        <div className="mt-3 text-xs text-muted-foreground space-y-0.5">
          <p>Input: {priceInput}</p>
          <p>Output: {priceOutput}</p>
        </div>
      </CardContent>
    </Card>
  )
}
```

- [ ] **Step 2: Rewrite models.tsx**

Fetch models from `/llm/models` API, display as card grid. Click card opens detail modal.

```tsx
// src/routes/models.tsx (full rewrite)
import { useEffect, useState } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { Navbar } from "@/components/navbar"
import { Footer } from "@/components/footer"
import { ModelCard } from "@/components/model-card"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog"
import { apiGet } from "@/api/client"

export const Route = createFileRoute("/models")({
  component: ModelsPage,
})

interface ModelInfo {
  id: number
  model_name: string
  provider_name: string
  capability: string
  price_input_per_1k: number
  price_output_per_1k: number
  status: string
}

function ModelsPage() {
  const [models, setModels] = useState<ModelInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [selected, setSelected] = useState<ModelInfo | null>(null)

  useEffect(() => {
    apiGet<ModelInfo[]>("/llm/models")
      .then(setModels)
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  return (
    <div className="min-h-screen flex flex-col">
      <Navbar />
      <main className="flex-1 container mx-auto px-4 py-10">
        <h1 className="text-3xl font-bold mb-2">Model Factory</h1>
        <p className="text-muted-foreground mb-8">Browse available models and their pricing.</p>

        {loading ? (
          <p className="text-muted-foreground">Loading models...</p>
        ) : models.length === 0 ? (
          <p className="text-muted-foreground">No models available yet.</p>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
            {models.map((m) => (
              <ModelCard
                key={m.id}
                name={m.model_name}
                provider={m.provider_name}
                capabilities={[m.capability]}
                priceInput={`$${m.price_input_per_1k.toFixed(4)}/1K tokens`}
                priceOutput={`$${m.price_output_per_1k.toFixed(4)}/1K tokens`}
                status={m.status}
                onClick={() => setSelected(m)}
              />
            ))}
          </div>
        )}
      </main>
      <Footer />

      {/* Detail dialog */}
      <Dialog open={!!selected} onOpenChange={(open) => !open && setSelected(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{selected?.model_name}</DialogTitle>
            <DialogDescription>Provider: {selected?.provider_name}</DialogDescription>
          </DialogHeader>
          {selected && (
            <div className="space-y-3 text-sm">
              <p><span className="font-medium">Capability:</span> {selected.capability}</p>
              <p><span className="font-medium">Status:</span> {selected.status}</p>
              <p><span className="font-medium">Input price:</span> ${selected.price_input_per_1k.toFixed(4)} / 1K tokens</p>
              <p><span className="font-medium">Output price:</span> ${selected.price_output_per_1k.toFixed(4)} / 1K tokens</p>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
```

- [ ] **Step 3: Verify TypeScript**

```bash
cd /d/code/github/ai-platform/app/web && npx tsc --noEmit 2>&1 | head -10
```

- [ ] **Step 4: Commit**

```bash
git add app/web/src/routes/models.tsx app/web/src/components/model-card.tsx
git commit -m "feat: rewrite model factory page with card grid and detail dialog"
```

---

### Task 5: Create Providers page (/providers)

**Files:**
- Create: `app/web/src/routes/providers.tsx`
- Create: `app/web/src/components/provider-card.tsx`

- [ ] **Step 1: Create ProviderCard component**

```tsx
// src/components/provider-card.tsx
import { Card, CardContent } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"

interface ProviderCardProps {
  name: string
  modelCount: number
  description?: string
}

export function ProviderCard({ name, modelCount, description }: ProviderCardProps) {
  return (
    <Card className="hover:shadow-md transition-shadow">
      <CardContent className="p-5">
        <p className="font-semibold">{name}</p>
        <Badge variant="secondary" className="mt-2 text-xs">{modelCount} models</Badge>
        {description && <p className="text-xs text-muted-foreground mt-2">{description}</p>}
      </CardContent>
    </Card>
  )
}
```

- [ ] **Step 2: Create providers.tsx route**

Top CTA banner for "Become a Provider" + provider card grid.

```tsx
// src/routes/providers.tsx
import { useEffect, useState } from "react"
import { createFileRoute, Link } from "@tanstack/react-router"
import { Navbar } from "@/components/navbar"
import { Footer } from "@/components/footer"
import { Button } from "@/components/ui/button"
import { ProviderCard } from "@/components/provider-card"
import { apiGet } from "@/api/client"

export const Route = createFileRoute("/providers")({
  component: ProvidersPage,
})

interface ProviderInfo {
  id: number
  name: string
  description?: string
  model_count: number
}

function ProvidersPage() {
  const [providers, setProviders] = useState<ProviderInfo[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    apiGet<ProviderInfo[]>("/llm/providers")
      .then(setProviders)
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  return (
    <div className="min-h-screen flex flex-col">
      <Navbar />
      <main className="flex-1">
        {/* CTA Banner */}
        <section className="py-16 bg-gradient-to-r from-primary/5 to-primary/10 text-center px-4">
          <h1 className="text-3xl font-bold">Become a Provider</h1>
          <p className="mt-3 text-muted-foreground max-w-xl mx-auto">
            Offer your AI models to thousands of developers. Simple integration, transparent revenue sharing.
          </p>
          <Link to="/providers/apply">
            <Button className="mt-6" size="lg">Apply Now</Button>
          </Link>
        </section>

        {/* Provider List */}
        <section className="container mx-auto px-4 py-12">
          <h2 className="text-2xl font-bold mb-6">Our Providers</h2>
          {loading ? (
            <p className="text-muted-foreground">Loading providers...</p>
          ) : providers.length === 0 ? (
            <p className="text-muted-foreground">No providers yet.</p>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
              {providers.map((p) => (
                <ProviderCard key={p.id} name={p.name} modelCount={p.model_count} description={p.description} />
              ))}
            </div>
          )}
        </section>
      </main>
      <Footer />
    </div>
  )
}
```

- [ ] **Step 3: Verify TypeScript**

```bash
cd /d/code/github/ai-platform/app/web && npx tsc --noEmit 2>&1 | head -10
```

- [ ] **Step 4: Commit**

```bash
git add app/web/src/routes/providers.tsx app/web/src/components/provider-card.tsx
git commit -m "feat: create providers page with CTA banner and provider card grid"
```

---

### Task 6: Create Provider Application form

**Files:**
- Create: `app/web/src/routes/providers.apply.tsx`

- [ ] **Step 1: Create providers.apply.tsx**

```tsx
// src/routes/providers.apply.tsx
import { useState } from "react"
import { createFileRoute, useNavigate } from "@tanstack/react-router"
import { Navbar } from "@/components/navbar"
import { Footer } from "@/components/footer"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { useAuthStore } from "@/stores/auth"
import { apiPost } from "@/api/client"

export const Route = createFileRoute("/providers/apply")({
  component: ApplyPage,
})

function ApplyPage() {
  const token = useAuthStore((s) => s.token)
  const navigate = useNavigate()
  const [submitting, setSubmitting] = useState(false)
  const [done, setDone] = useState(false)
  const [error, setError] = useState("")
  const [form, setForm] = useState({
    name: "",
    email: "",
    phone: "",
    telegram: "",
    wechat: "",
    models: "",
    notes: "",
  })

  const handleChange = (field: string) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    setForm((prev) => ({ ...prev, [field]: e.target.value }))
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!token) {
      navigate({ to: "/login" })
      return
    }
    setSubmitting(true)
    setError("")
    try {
      await apiPost("/provider/apply", form, token)
      setDone(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Submission failed")
    } finally {
      setSubmitting(false)
    }
  }

  if (done) {
    return (
      <div className="min-h-screen flex flex-col">
        <Navbar />
        <main className="flex-1 flex items-center justify-center px-4">
          <Card className="max-w-md w-full text-center">
            <CardContent className="p-8">
              <p className="text-2xl mb-2">🎉</p>
              <h2 className="text-xl font-bold">Application Submitted</h2>
              <p className="text-sm text-muted-foreground mt-2">We'll review your application and get back to you.</p>
              <Button className="mt-6" onClick={() => navigate({ to: "/" })}>Back to Home</Button>
            </CardContent>
          </Card>
        </main>
        <Footer />
      </div>
    )
  }

  return (
    <div className="min-h-screen flex flex-col">
      <Navbar />
      <main className="flex-1 container mx-auto px-4 py-10 max-w-2xl">
        <h1 className="text-3xl font-bold mb-2">Apply to Become a Provider</h1>
        <p className="text-muted-foreground mb-8">Fill out the form below and our team will review your application.</p>

        <Card>
          <CardHeader>
            <CardTitle>Provider Application</CardTitle>
            <CardDescription>All fields marked with * are required.</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="name">Enterprise/Individual Name *</Label>
                <Input id="name" required value={form.name} onChange={handleChange("name")} placeholder="Your name or company" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="email">Email *</Label>
                <Input id="email" type="email" required value={form.email} onChange={handleChange("email")} placeholder="contact@example.com" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="phone">Phone</Label>
                <Input id="phone" value={form.phone} onChange={handleChange("phone")} placeholder="+86 138 0000 0000" />
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="telegram">Telegram</Label>
                  <Input id="telegram" value={form.telegram} onChange={handleChange("telegram")} placeholder="@username" />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="wechat">WeChat</Label>
                  <Input id="wechat" value={form.wechat} onChange={handleChange("wechat")} placeholder="WeChat ID" />
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="models">Models/Services You Plan to Provide *</Label>
                <Textarea id="models" required value={form.models} onChange={handleChange("models")} placeholder="Describe the models or services you want to offer..." rows={3} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="notes">Notes (Optional)</Label>
                <Textarea id="notes" value={form.notes} onChange={handleChange("notes")} rows={2} />
              </div>

              {error && <p className="text-sm text-red-500">{error}</p>}

              <Button type="submit" disabled={submitting} className="w-full">
                {submitting ? "Submitting..." : "Submit Application"}
              </Button>
            </form>
          </CardContent>
        </Card>
      </main>
      <Footer />
    </div>
  )
}
```

- [ ] **Step 2: Verify TypeScript**

```bash
cd /d/code/github/ai-platform/app/web && npx tsc --noEmit 2>&1 | head -10
```

- [ ] **Step 3: Commit**

```bash
git add app/web/src/routes/providers.apply.tsx
git commit -m "feat: create provider application form page"
```

---

### Task 7: Create Console layout with sidebar + tab routing

**Files:**
- Create: `app/web/src/routes/console.tsx`
- Create: `app/web/src/components/console-layout.tsx`
- Delete (later): `app/web/src/routes/dashboard.tsx` (replaced by console/index.tsx)
- Delete (later): `app/web/src/routes/transactions.tsx` (removed from spec)

- [ ] **Step 1: Create ConsoleLayout component**

```tsx
// src/components/console-layout.tsx
import { Link, Outlet, useLocation } from "@tanstack/react-router"
import { useAuthStore } from "@/stores/auth"
import { cn } from "@/lib/utils"
import { LayoutDashboard, Key, BarChart3, Receipt, LogOut } from "lucide-react"

const tabs = [
  { to: "/console", label: "Overview", icon: LayoutDashboard, exact: true },
  { to: "/console/keys", label: "API Keys", icon: Key },
  { to: "/console/usage", label: "Usage", icon: BarChart3 },
  { to: "/console/orders", label: "Orders", icon: Receipt },
]

export function ConsoleLayout() {
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)
  const location = useLocation()

  const isActive = (tab: typeof tabs[0]) => {
    if (tab.exact) return location.pathname === tab.to
    return location.pathname.startsWith(tab.to)
  }

  return (
    <div className="min-h-screen flex">
      {/* Sidebar */}
      <aside className="w-56 border-r bg-muted/20 flex flex-col">
        <div className="p-4 border-b">
          <Link to="/" className="font-bold">AI Platform</Link>
        </div>
        <nav className="flex-1 p-3 space-y-1">
          {tabs.map((tab) => (
            <Link
              key={tab.to}
              to={tab.to}
              className={cn(
                "flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors",
                isActive(tab)
                  ? "bg-primary text-primary-foreground"
                  : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
              )}
            >
              <tab.icon className="h-4 w-4" />
              {tab.label}
            </Link>
          ))}
        </nav>
        <div className="p-3 border-t">
          <div className="flex items-center justify-between">
            <span className="text-sm text-muted-foreground truncate">{user?.username}</span>
            <button onClick={logout} className="text-muted-foreground hover:text-foreground">
              <LogOut className="h-4 w-4" />
            </button>
          </div>
        </div>
      </aside>

      {/* Content */}
      <main className="flex-1">
        <Outlet />
      </main>
    </div>
  )
}
```

- [ ] **Step 2: Create console.tsx route (parent layout)**

```tsx
// src/routes/console.tsx
import { createFileRoute, Navigate } from "@tanstack/react-router"
import { ConsoleLayout } from "@/components/console-layout"
import { useAuthStore } from "@/stores/auth"

export const Route = createFileRoute("/console")({
  component: ConsoleRoute,
})

function ConsoleRoute() {
  const token = useAuthStore((s) => s.token)
  if (!token) return <Navigate to="/login" />
  return <ConsoleLayout />
}
```

Note: TanStack Router file-based routing convention — `console.tsx` creates path `/console`. Child routes (`console.keys.tsx`, etc.) get `getParentRoute: () => ConsoleRoute`. The TypeScript router plugin auto-generates this from the file naming convention.

- [ ] **Step 3: Verify TypeScript**

```bash
cd /d/code/github/ai-platform/app/web && npx tsc --noEmit 2>&1 | head -10
```

- [ ] **Step 4: Commit**

```bash
git add app/web/src/routes/console.tsx app/web/src/components/console-layout.tsx
git commit -m "feat: create console layout with sidebar navigation"
```

---

### Task 8: Create Console Overview tab

**Files:**
- Create: `app/web/src/routes/console.index.tsx`
- Delete: `app/web/src/routes/dashboard.tsx` (old, replaced by console overview)

- [ ] **Step 1: Create console.index.tsx**

```tsx
// src/routes/console.index.tsx
import { createFileRoute } from "@tanstack/react-router"
import { useEffect, useState } from "react"
import { useAuthStore } from "@/stores/auth"
import { apiGet } from "@/api/client"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

export const Route = createFileRoute("/console/")({
  component: ConsoleOverview,
})

interface AccountInfo {
  owner_type: string
  owner_id: number
  asset: string
  balance_micro: number
}

interface UsageStats {
  total_calls: number
  total_tokens: number
  total_credits: number
  avg_latency_ms: number
}

function ConsoleOverview() {
  const token = useAuthStore((s) => s.token)
  const user = useAuthStore((s) => s.user)
  const [account, setAccount] = useState<AccountInfo | null>(null)
  const [stats, setStats] = useState<UsageStats | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!token) return
    Promise.all([
      apiGet<AccountInfo>("/billing/balance", token),
      apiGet<UsageStats>("/usage/stats", token),
    ])
      .then(([acc, st]) => {
        setAccount(acc)
        setStats(st)
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [token])

  const balance = account ? (account.balance_micro / 1_000_000).toFixed(2) : "0.00"

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-2xl font-bold">Overview</h1>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm text-muted-foreground">Balance</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{loading ? "..." : `${balance} ${account?.asset || ""}`}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm text-muted-foreground">Total Calls</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{loading ? "..." : stats?.total_calls?.toLocaleString() ?? "0"}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm text-muted-foreground">Total Tokens</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{loading ? "..." : stats?.total_tokens?.toLocaleString() ?? "0"}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm text-muted-foreground">Total Credits</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{loading ? "..." : (stats?.total_credits ?? 0).toFixed(2)}</p>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Account</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-1">
          <p>Username: {user?.username}</p>
          <p>Email: {user?.email}</p>
          <p>User ID: {user?.id}</p>
        </CardContent>
      </Card>
    </div>
  )
}
```

- [ ] **Step 2: Delete old dashboard.tsx**

```bash
rm /d/code/github/ai-platform/app/web/src/routes/dashboard.tsx
```

- [ ] **Step 3: Verify TypeScript**

```bash
cd /d/code/github/ai-platform/app/web && npx tsc --noEmit 2>&1 | head -10
```

- [ ] **Step 4: Commit**

```bash
git add app/web/src/routes/console.index.tsx
git add app/web/src/routes/dashboard.tsx  # staged for deletion
git commit -m "feat: create console overview tab, remove old dashboard route"
```

---

### Task 9: Integrate Keys, Usage, Orders into Console

**Files:**
- Create: `app/web/src/routes/console.keys.tsx`
- Create: `app/web/src/routes/console.usage.tsx`
- Create: `app/web/src/routes/console.orders.tsx`
- Delete: `app/web/src/routes/keys.tsx`
- Delete: `app/web/src/routes/usage.tsx`
- Delete: `app/web/src/routes/orders.tsx`
- Delete: `app/web/src/routes/transactions.tsx`

- [ ] **Step 1: Create console.keys.tsx**

Copy content from existing `keys.tsx`, update route definition:

```tsx
// src/routes/console.keys.tsx
import { createFileRoute } from "@tanstack/react-router"
// ... rest is identical to original keys.tsx
```

The only change:
```tsx
export const Route = createFileRoute("/console/keys")({
  component: KeysPage,
})
```

Everything else (interfaces, component code) stays the same.

- [ ] **Step 2: Create console.usage.tsx**

Copy from existing `usage.tsx`, update route:
```tsx
export const Route = createFileRoute("/console/usage")({
  component: UsagePage,
})
```

- [ ] **Step 3: Create console.orders.tsx (repurpose for recharge)**

Rewrite the existing orders page to show recharge orders. Update API endpoint to `/billing/orders`:

```tsx
// src/routes/console.orders.tsx
import { createFileRoute } from "@tanstack/react-router"
import { useEffect, useState } from "react"
import { useAuthStore } from "@/stores/auth"
import { apiGet } from "@/api/client"

export const Route = createFileRoute("/console/orders")({
  component: OrdersPage,
})

interface OrderInfo {
  order_no: string
  status: string
  amount_credits: number
  amount_usd: number
  created_at: string
}

function OrdersPage() {
  const token = useAuthStore((s) => s.token)
  const [orders, setOrders] = useState<OrderInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")

  useEffect(() => {
    if (!token) return
    apiGet<OrderInfo[]>("/billing/orders", token)
      .then(setOrders)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }, [token])

  return (
    <div className="p-6 space-y-4">
      <h1 className="text-2xl font-bold">Orders</h1>
      <p className="text-sm text-muted-foreground">Recharge orders. Completed orders are credited to your balance immediately.</p>

      {loading && <p className="text-muted-foreground">Loading...</p>}
      {error && <p className="text-red-500 text-sm">{error}</p>}

      {!loading && !error && orders.length === 0 && (
        <p className="text-muted-foreground">No orders yet.</p>
      )}

      {orders.length > 0 && (
        <div className="border rounded-md overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-muted/50">
              <tr>
                <th className="text-left p-3 font-medium">Order No.</th>
                <th className="text-left p-3 font-medium">Status</th>
                <th className="text-right p-3 font-medium">Amount (USD)</th>
                <th className="text-right p-3 font-medium">Credits</th>
                <th className="text-left p-3 font-medium">Created At</th>
              </tr>
            </thead>
            <tbody>
              {orders.map((o) => (
                <tr key={o.order_no} className="border-t">
                  <td className="p-3 font-mono text-xs">{o.order_no}</td>
                  <td className="p-3">
                    <span className={`text-xs px-2 py-1 rounded capitalize ${
                      o.status === "completed" ? "bg-green-100 text-green-700" : "bg-yellow-100 text-yellow-700"
                    }`}>
                      {o.status}
                    </span>
                  </td>
                  <td className="p-3 text-right">${(o.amount_usd ?? 0).toFixed(2)}</td>
                  <td className="p-3 text-right">{o.amount_credits?.toLocaleString()}</td>
                  <td className="p-3 text-muted-foreground">{new Date(o.created_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 4: Delete old flat routes**

```bash
rm /d/code/github/ai-platform/app/web/src/routes/keys.tsx
rm /d/code/github/ai-platform/app/web/src/routes/usage.tsx
rm /d/code/github/ai-platform/app/web/src/routes/orders.tsx
rm /d/code/github/ai-platform/app/web/src/routes/transactions.tsx
```

- [ ] **Step 5: Regenerate route tree and verify**

```bash
cd /d/code/github/ai-platform/app/web && npm run typecheck 2>&1 | head -20
```

Expected: TypeScript passes, TanStack Router regenerates routeTree.gen.ts automatically (via Vite plugin).

- [ ] **Step 6: Commit**

```bash
git add app/web/src/routes/console.keys.tsx app/web/src/routes/console.usage.tsx app/web/src/routes/console.orders.tsx
git add app/web/src/routes/keys.tsx app/web/src/routes/usage.tsx app/web/src/routes/orders.tsx app/web/src/routes/transactions.tsx
git commit -m "refactor: integrate keys, usage, orders into console; repurpose orders for recharge"
```

---

### Task 10: Update root layout for new navigation

**Files:**
- Modify: `app/web/src/routes/__root.tsx`

- [ ] **Step 1: Simplify root layout**

The root layout currently has a logged-in-only header with nav links. Since public pages now have Navbar component and Console has its own sidebar, the root layout should be minimal:

```tsx
// src/routes/__root.tsx (rewrite)
import { createRootRoute, Outlet } from "@tanstack/react-router"
import { useEffect } from "react"
import { useAuthStore } from "@/stores/auth"

export const Route = createRootRoute({
  component: RootLayout,
})

function RootLayout() {
  const token = useAuthStore((s) => s.token)
  const user = useAuthStore((s) => s.user)
  const fetchProfile = useAuthStore((s) => s.fetchProfile)

  useEffect(() => {
    if (token && !user) {
      fetchProfile()
    }
  }, [token, user, fetchProfile])

  return (
    <div className="min-h-screen bg-background">
      <Outlet />
    </div>
  )
}
```

- [ ] **Step 2: Verify**

```bash
cd /d/code/github/ai-platform/app/web && npx tsc --noEmit 2>&1 | head -10
```

- [ ] **Step 3: Commit**

```bash
git add app/web/src/routes/__root.tsx
git commit -m "refactor: simplify root layout, move nav to per-page components"
```

---

### Task 11: Backend — Provider application API endpoint

**Files:**
- Create: (new migration) `server/migrations/0006_provider_application.sql`
- Create: `server/internal/controller/api/provider.go`
- Modify: `server/internal/logic/logic.go` (import new logic package)

- [ ] **Step 1: Migration for provider_applications table**

```sql
-- server/migrations/0006_provider_application.sql
-- +goose Up
CREATE TABLE provider_applications (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL DEFAULT 0,
    name TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    telegram TEXT NOT NULL DEFAULT '',
    wechat TEXT NOT NULL DEFAULT '',
    models TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    status SMALLINT NOT NULL DEFAULT 0,   -- 0=pending, 1=approved, 2=rejected
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS provider_applications;
```

- [ ] **Step 2: Create provider logic**

```go
// server/internal/logic/provider/provider.go
package provider

import (
    "context"
    "github.com/gogf/gf/v2/database/gdb"
    "github.com/gogf/gf/v2/errors/gerror"
    "github.com/gogf/gf/v2/frame/g"
    "github.com/tiger1103/ai-platform/server/internal/service"
)

type sProvider struct{}

func init() {
    service.RegisterProvider(New())
}

func New() *sProvider {
    return &sProvider{}
}

func (s *sProvider) Apply(ctx context.Context, in service.ProviderApplyInput) error {
    _, err := g.DB().Model("provider_applications").Ctx(ctx).Insert(g.Map{
        "user_id":  in.UserID,
        "name":     in.Name,
        "email":    in.Email,
        "phone":    in.Phone,
        "telegram": in.Telegram,
        "wechat":   in.WeChat,
        "models":   in.Models,
        "notes":    in.Notes,
    })
    if err != nil {
        return gerror.Wrap(err, "submit provider application")
    }
    return nil
}
```

- [ ] **Step 3: Register service interface**

```go
// server/internal/service/provider.go
package service

import "context"

type ProviderApplyInput struct {
    UserID   int64
    Name     string
    Email    string
    Phone    string
    Telegram string
    WeChat   string
    Models   string
    Notes    string
}

type IProvider interface {
    Apply(ctx context.Context, in ProviderApplyInput) error
}

var localProvider IProvider

func RegisterProvider(i IProvider) { localProvider = i }
func Provider() IProvider { return localProvider }
```

- [ ] **Step 4: Create API controller**

```go
// server/internal/controller/api/provider.go
package api

import (
    "context"
    "github.com/gogf/gf/v2/errors/gerror"
    "github.com/gogf/gf/v2/frame/g"
    "github.com/gogf/gf/v2/net/ghttp"
    "github.com/tiger1103/ai-platform/server/internal/service"
)

type ProviderController struct{}

func (c *ProviderController) Apply(r *ghttp.Request) {
    var (
        ctx  = r.GetCtx()
        user = g.RequestFromCtx(ctx).GetCtxVar("user")
    )
    userID := user.Map()["id"].(int64)

    in := service.ProviderApplyInput{
        UserID:   userID,
        Name:     r.Get("name").String(),
        Email:    r.Get("email").String(),
        Phone:    r.Get("phone").String(),
        Telegram: r.Get("telegram").String(),
        WeChat:   r.Get("wechat").String(),
        Models:   r.Get("models").String(),
        Notes:    r.Get("notes").String(),
    }

    if in.Name == "" || in.Email == "" || in.Models == "" {
        gerror.Throw("name, email, and models are required")
    }

    if err := service.Provider().Apply(ctx, in); err != nil {
        r.Response.WriteJson(g.Map{"code": -1, "message": err.Error()})
        return
    }
    r.Response.WriteJson(g.Map{"code": 0, "message": "ok"})
}
```

- [ ] **Step 5: Register route + logic import**

Add route in the API server's route registration:
```go
// In api route setup
s.Group("/api/v1/provider", func(group *ghttp.RouterGroup) {
    group.POST("/apply", "ProviderController.Apply")
})
```

Add blank import in `logic/logic.go`:
```go
import (
    _ "github.com/tiger1103/ai-platform/server/internal/logic/provider"
)
```

- [ ] **Step 6: Run migration**

```bash
cd /d/code/github/ai-platform/server && goose up
```

Expected: migration 0006 applied, provider_applications table created.

- [ ] **Step 7: Verify build**

```bash
cd /d/code/github/ai-platform/server && go build ./...
```

Expected: success.

- [ ] **Step 8: Commit**

```bash
git add server/migrations/0006_provider_application.sql
git add server/internal/controller/api/provider.go
git add server/internal/service/provider.go
git add server/internal/logic/provider/
git commit -m "feat: add provider application API endpoint and migration"
```
