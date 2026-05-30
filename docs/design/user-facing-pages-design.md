# AI Platform User-Facing Pages Design

## Overview

Four pages for the ai-platform web frontend (React SPA): Home, Model Factory, Providers, Console. Two roles: user (`role: 0`) and provider (`role: 1`). Provider inherits all user views plus provider-specific data.

## Pages

### 1. Home (落地页 / Landing Page)

| | |
|---|---|
| Type | Public (no auth required) |
| Route | `/` |

Sections:
- **Hero** — One-line positioning + CTA button ("Start Now" / "Register")
- **Model Showcase** — Grid of supported models with icon, name, brief description
- **Core Advantages** — 3-4 cards: High Availability, Transparent Pricing, Low Latency, Developer Friendly
- **Provider CTA** — Section encouraging providers to join, link to /providers
- **Footer** — Links, contact

### 2. Model Factory (模型工厂)

| | |
|---|---|
| Type | Public (browsable without login) |
| Route | `/models` |

Model card grid showing all available models. Each card:
- Model name
- Provider name
- Capabilities: chat / embedding / image (tags)
- Price per 1K tokens (input / output)
- Status: active / maintenance

Click card → detail panel/modal with: model description, pricing breakdown, code example.

### 3. Providers (供应商)

| | |
|---|---|
| Type | Public (browsable without login) |
| Route | `/providers` |

Provider card grid. Each card:
- Provider name / logo
- Number of models provided
- "View Models" link

Top of page: CTA banner — "Become a Provider" → registration/application flow.

### 4. Console (控制台)

| | |
|---|---|
| Type | Auth required |
| Route | `/console` |
| Layout | Sidebar menu + tab content area |

**Sidebar tabs:**

| Tab | Route | User sees | Provider additionally sees |
|-----|-------|-----------|--------------------------|
| Overview | `/console` | Balance + credit card + today usage (requests, tokens) + recent transactions | Same + today income + provider model stats |
| API Keys | `/console/keys` | Manage API keys (create, revoke) | Same |
| Usage | `/console/usage` | Usage charts (requests, tokens, cost over time) | Same + provider model usage |
| Orders | `/console/orders` | Recharge order history (order no, amount, status, time). Recharge converts immediately to credits. | Same |

**Left sidebar navigation** with top-aligned menu items. Active tab highlighted.

**Top bar** (within Console layout, above tab content):
- Balance / credit display
- "Apply to become Provider" CTA (visible to regular users only)

### Existing Routes Mapping

| Existing file | Becomes |
|---|---|
| `routes/dashboard.tsx` | Console → Overview tab content |
| `routes/keys.tsx` | Console → API Keys tab content |
| `routes/usage.tsx` | Console → Usage tab content |
| `routes/models.tsx` | Model Factory page |
| `routes/orders.tsx` | Console → Orders tab content (repurpose from marketplace orders) |

## Implementation Notes

- Use TanStack Router nested layouts for Console sidebar
- Existing route files stay as individual components, Console layout wraps them via outlet/fragment
- Provider-specific sections conditionally rendered based on `user.role === 1`

---

## Appendix: Provider Application Flow

### Entry
Top section of `/providers` page — CTA banner: "Become a Provider"

### Form Fields
- Enterprise name or individual name
- Email
- Phone
- Telegram handle
- WeChat handle
- Models/services you plan to provide (free text)
- Notes (optional)

### Backend Flow
1. User submits form → POST to new API endpoint `/api/v1/provider/apply`
2. Creates a provider application record (pending review)
3. Admin reviews via gfast backend (:8082)
4. On approval: user `role` updated from `0` → `1`
5. Notification sent to applicant
