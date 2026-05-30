import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { Navbar } from '../components/Navbar'
import { Footer } from '../components/Footer'

export const Route = createFileRoute('/providers-apply')({
  component: ApplyPage,
})

const STEPS = ['Basic Info', 'Technical Details', 'Review & Submit']

function ApplyPage() {
  const [step, setStep] = useState(0)
  const [submitted, setSubmitted] = useState(false)
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  const [form, setForm] = useState({
    company: '', contact: '', email: '', website: '', bio: '',
    modelName: '', modelFamily: '', apiEndpoint: '', documentation: '',
  })

  const update = (field: string, value: string) => setForm((f) => ({ ...f, [field]: value }))

  const handleSubmit = async () => {
    setLoading(true)
    try {
      const res = await fetch('/api/v1/provider/apply', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          company: form.company, contact: form.contact, email: form.email,
          website: form.website, bio: form.bio,
          model_name: form.modelName, model_family: form.modelFamily,
          api_endpoint: form.apiEndpoint, documentation: form.documentation,
        }),
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.message || 'Submission failed')
      setSubmitted(true)
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Submission failed')
    } finally {
      setLoading(false)
    }
  }

  if (submitted) {
    return (
      <div className="min-h-screen bg-background">
        <Navbar />
        <div className="mx-auto max-w-lg px-4 py-24 text-center">
          <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-primary text-3xl text-primary-foreground">✓</div>
          <h1 className="mt-6 text-3xl font-bold">Application Submitted</h1>
          <p className="mt-3 text-muted-foreground">We'll review your application and get back to you within 3-5 business days.</p>
          <a href="/providers" className="mt-8 inline-flex h-10 items-center justify-center rounded-lg bg-primary px-6 text-sm font-medium text-primary-foreground">
            Back to Providers
          </a>
        </div>
        <Footer />
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-background">
      <Navbar />
      <div className="mx-auto max-w-2xl px-4 py-12">
        <h1 className="text-3xl font-bold tracking-tight">Apply as a Provider</h1>

        {/* Step indicator */}
        <div className="mt-8 flex items-center gap-2">
          {STEPS.map((s, i) => (
            <div key={s} className="flex items-center gap-2">
              <div className={`flex h-8 w-8 items-center justify-center rounded-full text-xs font-bold ${i <= step ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'}`}>
                {i + 1}
              </div>
              <span className={`text-sm ${i <= step ? 'text-foreground' : 'text-muted-foreground'}`}>{s}</span>
              {i < STEPS.length - 1 && <div className={`h-px w-8 ${i < step ? 'bg-primary' : 'bg-border'}`} />}
            </div>
          ))}
        </div>

        {/* Step 1: Basic Info */}
        {step === 0 && (
          <div className="mt-8 space-y-4">
            <div>
              <label className="block text-sm font-medium mb-1">Company Name *</label>
              <input value={form.company} onChange={(e) => update('company', e.target.value)} className="h-10 w-full rounded-lg border border-input bg-card px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" placeholder="Your company name" />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">Contact Person *</label>
              <input value={form.contact} onChange={(e) => update('contact', e.target.value)} className="h-10 w-full rounded-lg border border-input bg-card px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" placeholder="Full name" />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">Email *</label>
              <input type="email" value={form.email} onChange={(e) => update('email', e.target.value)} className="h-10 w-full rounded-lg border border-input bg-card px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" placeholder="contact@company.com" />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">Website</label>
              <input value={form.website} onChange={(e) => update('website', e.target.value)} className="h-10 w-full rounded-lg border border-input bg-card px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" placeholder="https://" />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">Company Bio</label>
              <textarea value={form.bio} onChange={(e) => update('bio', e.target.value)} rows={3} className="w-full rounded-lg border border-input bg-card px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring" placeholder="Tell us about your company..." />
            </div>
          </div>
        )}

        {/* Step 2: Technical */}
        {step === 1 && (
          <div className="mt-8 space-y-4">
            <div>
              <label className="block text-sm font-medium mb-1">Model Name *</label>
              <input value={form.modelName} onChange={(e) => update('modelName', e.target.value)} className="h-10 w-full rounded-lg border border-input bg-card px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" placeholder="e.g. gpt-4o" />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">Model Family</label>
              <input value={form.modelFamily} onChange={(e) => update('modelFamily', e.target.value)} className="h-10 w-full rounded-lg border border-input bg-card px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" placeholder="e.g. GPT-4" />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">API Endpoint</label>
              <input value={form.apiEndpoint} onChange={(e) => update('apiEndpoint', e.target.value)} className="h-10 w-full rounded-lg border border-input bg-card px-3 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-ring" placeholder="https://api.example.com/v1" />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">Documentation URL</label>
              <input value={form.documentation} onChange={(e) => update('documentation', e.target.value)} className="h-10 w-full rounded-lg border border-input bg-card px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring" placeholder="https://docs.example.com" />
            </div>
          </div>
        )}

        {/* Step 3: Review */}
        {step === 2 && (
          <div className="mt-8 space-y-3">
            <h3 className="font-semibold">Review Your Application</h3>
            {[
              ['Company', form.company],
              ['Contact', form.contact],
              ['Email', form.email],
              ['Website', form.website],
              ['Model', form.modelName],
              ['Family', form.modelFamily],
              ['API Endpoint', form.apiEndpoint],
            ].filter(([, v]) => v).map(([label, value]) => (
              <div key={label as string} className="flex justify-between rounded-lg bg-muted px-4 py-3 text-sm">
                <span className="text-muted-foreground">{label as string}</span>
                <span className="font-medium">{value as string}</span>
              </div>
            ))}
          </div>
        )}

        {/* Navigation */}
        <div className="mt-8 flex justify-between">
          {step > 0 ? (
            <button onClick={() => setStep(step - 1)} className="h-10 rounded-lg border border-border bg-card px-6 text-sm font-medium transition-colors hover:bg-muted">
              Back
            </button>
          ) : <div />}
          {step < STEPS.length - 1 ? (
            <button onClick={() => setStep(step + 1)} className="h-10 rounded-lg bg-primary px-6 text-sm font-medium text-primary-foreground transition-opacity hover:opacity-90">
              Next →
            </button>
          ) : (
            <button onClick={handleSubmit} disabled={loading} className="h-10 rounded-lg bg-primary px-6 text-sm font-medium text-primary-foreground transition-opacity hover:opacity-90 disabled:opacity-50">
              {loading ? 'Submitting...' : 'Submit Application'}
            </button>
          )}
        </div>
      </div>
      <Footer />
    </div>
  )
}
