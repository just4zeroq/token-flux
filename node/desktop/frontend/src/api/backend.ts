// API layer — calls Wails runtime, falls back to HTTP in standalone dev mode.
// Production (Wails) path: frontend → callWails → Go backend → DB / platform
// Dev browser path:       frontend → HTTP → localhost:20128 REST API

const DEV_BASE = 'http://localhost:20128'

async function callWails(method: string, ...args: any[]): Promise<any> {
  // Wails v2 standard binding: window.go.main.NodeService.method(...)
  try {
    const ns = (window as any)?.go?.main?.NodeService
    if (ns && typeof ns[method] === 'function') {
      return ns[method](...args)
    }
  } catch {}
  // Fallback for older Wails versions
  try {
    if ((window as any).runtime?.Call) {
      return (window as any).runtime.Call(method, ...args)
    }
  } catch {}
  return null
}

// Fallback HTTP helpers for standalone dev mode (no Wails runtime)
async function _httpGet(path: string): Promise<any> {
  try { const r = await fetch(`${DEV_BASE}${path}`); return r.json() } catch { return null }
}
async function _httpPost(path: string, body: any): Promise<any> {
  try {
    const r = await fetch(`${DEV_BASE}${path}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
    return r.status === 204 ? null : r.json()
  } catch { return null }
}
async function _httpPut(path: string, body: any): Promise<any> {
  try {
    const r = await fetch(`${DEV_BASE}${path}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
    return r.json()
  } catch { return null }
}
async function _httpDelete(path: string, body?: any): Promise<boolean> {
  try {
    const r = await fetch(`${DEV_BASE}${path}`, { method: 'DELETE', headers: { 'Content-Type': 'application/json' }, body: body ? JSON.stringify(body) : undefined })
    return r.status === 204 || r.ok
  } catch { return false }
}

// ---- Node status ----

export interface NodeStatus {
  running: boolean
  models_count: number
  keys_count?: number
  uptime_sec?: number
  total_calls?: number
  total_tokens?: number
}

export async function getStatus(): Promise<NodeStatus | null> {
  const w = await callWails('Status')
  if (w) return w
  return _httpGet('/api/node/status')
}

// ---- Upstream API keys ----

export async function listKeys(): Promise<any[]> {
  const w = await callWails('ListKeys')
  if (w) return w
  const r = await _httpGet('/api/node/keys')
  return r?.keys ?? []
}

export async function addKey(name: string, key: string, channelId: number, baseUrl: string): Promise<any> {
  const w = await callWails('AddKey', name, key, baseUrl, channelId)
  if (w) return w
  return _httpPost('/api/node/keys', { name, key, channel_id: channelId, base_url: baseUrl })
}

export async function deleteKey(id: number): Promise<boolean> {
  const w = await callWails('DeleteKey', id)
  if (w !== null) return true
  return _httpDelete('/api/node/keys', { id })
}

// ---- Key-Model bindings ----

export async function listBindings(): Promise<any[]> {
  const w = await callWails('ListBindings')
  if (w) return w
  const r = await _httpGet('/api/node/models')
  return r?.bindings ?? []
}

export async function bindModel(keyId: number, modelCode: string, upstreamModelName: string, shared: boolean): Promise<any> {
  const w = await callWails('BindModel', keyId, modelCode, upstreamModelName, shared)
  if (w) return w
  return _httpPost('/api/node/models/bind', { key_id: keyId, model_code: modelCode, upstream_model_name: upstreamModelName, shared })
}

export async function unbindModel(id: number): Promise<boolean> {
  const w = await callWails('UnbindModel', id)
  if (w !== null) return true
  return _httpDelete('/api/node/models', { id })
}

// ---- Combos ----

export async function listCombos(): Promise<any[]> {
  const w = await callWails('ListCombos')
  if (w) return w
  // no REST endpoint for combos
  return []
}

export async function createCombo(name: string, models: string[], strategy: string, sticky: number): Promise<any> {
  const w = await callWails('CreateCombo', name, models, strategy, sticky)
  if (w) return w
  return { error: 'Wails runtime required for combo creation' }
}

export async function deleteCombo(name: string): Promise<boolean> {
  const w = await callWails('DeleteCombo', name)
  if (w !== null) return true
  return false
}

// ---- Usage ----

export async function getUsageStats(): Promise<any> {
  const w = await callWails('GetUsageStats')
  if (w) return w
  const r = await _httpGet('/api/node/usage')
  return r?.summary ?? null
}

export async function getUsageLogs(limit?: number): Promise<any[]> {
  const w = await callWails('GetUsageLogs', limit ?? 50)
  if (w) return w
  const r = await _httpGet('/api/node/usage')
  return r?.recent ?? []
}

// ---- Local channels CRUD (via Go backend / REST fallback) ----

export async function listChannels(): Promise<any[]> {
  const w = await callWails('ListChannels')
  if (w) return w
  const r = await _httpGet('/api/node/channels')
  return r?.channels ?? []
}

export async function createChannel(code: string, name: string, description: string, providerType: number, protocols: any[]): Promise<any> {
  const w = await callWails('CreateChannel', code, name, description, providerType, protocols)
  if (w) return w
  return _httpPost('/api/node/channels', { code, name, description, provider_type: providerType, protocols })
}

export async function updateChannel(id: number, name: string, protocols: any[], status: string): Promise<any> {
  const w = await callWails('UpdateChannel', id, name, protocols, status)
  if (w) return w
  return _httpPut('/api/node/channels', { id, name, protocols, status })
}

export async function deleteChannel(id: number): Promise<boolean> {
  const w = await callWails('DeleteChannel', id)
  if (w === 'deleted') return true
  return _httpDelete('/api/node/channels', { id })
}

export async function listChannelModels(channelId: number): Promise<any[]> {
  const w = await callWails('ListChannelModels', channelId)
  if (w) return w
  const r = await _httpGet('/api/node/channel-models?channel_id=' + channelId)
  return r?.models ?? []
}

export async function bindChannelModels(channelId: number, modelIds: number[]): Promise<boolean> {
  const w = await callWails('BindChannelModels', channelId, modelIds)
  if (w === 'ok') return true
  const r = await _httpPost('/api/node/channel-models', { channel_id: channelId, model_ids: modelIds })
  return r !== null
}

export async function unbindChannelModel(bindingId: number): Promise<boolean> {
  const w = await callWails('UnbindChannelModel', bindingId)
  if (w === 'ok') return true
  return _httpDelete('/api/node/channel-models', { id: bindingId })
}

// ---- Local model specs CRUD (via Go backend / REST fallback) ----

export async function listModelSpecs(): Promise<any[]> {
  const w = await callWails('ListModelSpecs')
  if (w) return w
  const r = await _httpGet('/api/node/model-specs')
  return r?.model_specs ?? []
}

export async function createModelSpec(spec: any): Promise<any> {
  const w = await callWails('CreateModelSpec', spec)
  if (w) return w
  return _httpPost('/api/node/model-specs', spec)
}

export async function deleteModelSpec(id: number): Promise<boolean> {
  const w = await callWails('DeleteModelSpec', id)
  if (w === 'deleted') return true
  return _httpDelete('/api/node/model-specs', { id })
}

// ---- Node API keys (via Go backend / REST fallback) ----

export async function listNodeApiKeys(): Promise<any[]> {
  const w = await callWails('ListNodeApiKeys')
  if (w) return w
  const r = await _httpGet('/api/node/api-keys')
  return r?.keys ?? []
}

export async function createNodeApiKey(label: string): Promise<any> {
  const w = await callWails('CreateNodeApiKey', label)
  if (w) return w
  return _httpPost('/api/node/api-keys', { label })
}

export async function deleteNodeApiKey(id: number): Promise<boolean> {
  const w = await callWails('DeleteNodeApiKey', id)
  if (w !== null) return true
  return _httpDelete('/api/node/api-keys', { id })
}

// ---- Wallet ----

export async function walletStatus(): Promise<any> {
  const w = await callWails('WalletStatus')
  if (w) return w
  return _httpGet('/api/node/wallet')
}

export async function setupWallet(): Promise<any> {
  const w = await callWails('SetupWallet')
  if (w) return w
  return _httpPost('/api/node/wallet', {})
}

// ---- Network ----

export async function createWallet(name: string): Promise<any> {
  const w = await callWails('CreateWallet', name)
  if (w) return w
  return { error: 'Wails runtime required' }
}

export async function joinNetwork(name?: string): Promise<string> {
  const w = await callWails('JoinNetwork', name ?? '')
  if (w) return w
  return 'error: Wails runtime required'
}

export async function connectToNetwork(): Promise<string> {
  const w = await callWails('ConnectToNetwork')
  if (w) return w
  return 'error: Wails runtime required'
}

export async function disconnectFromNetwork(): Promise<string> {
  const w = await callWails('DisconnectFromNetwork')
  if (w) return w
  return 'error: Wails runtime required'
}

export async function leaveNetwork(): Promise<string> {
  const w = await callWails('LeaveNetwork')
  if (w) return w
  return 'error: Wails runtime required'
}

export async function networkStatus(): Promise<any> {
  const w = await callWails('NetworkStatus')
  if (w) return w
  return _httpGet('/api/node/network')
}

export async function exportSeed(): Promise<any> {
  const w = await callWails('ExportSeed')
  if (w) return w
  return { error: 'Wails runtime required' }
}

export async function fetchPeerCount(): Promise<number> {
  const w = await callWails('FetchPeerCount')
  if (typeof w === 'number') return w
  return 0
}

// ---- Platform catalog (Wails + HTTP fallback) ----

const PLATFORM_URL = 'http://localhost:8080'

async function _fetchPlatform(path: string): Promise<any[]> {
  // Try Wails first
  let w: any = null
  if (path.includes('channels')) w = await callWails('FetchCatalogChannels')
  else if (path.includes('providers')) w = await callWails('FetchCatalogProviders')
  else w = await callWails('FetchCatalogModels')
  if (w) return w
  // HTTP fallback — direct to platform API
  try {
    const r = await fetch(`${PLATFORM_URL}${path}`, { signal: AbortSignal.timeout(5000) })
    const d = await r.json()
    return d?.data?.list ?? d ?? []
  } catch {
    return []
  }
}

export async function fetchCatalogProviders(): Promise<any[]> {
  return _fetchPlatform('/api/v1/catalog/providers')
}

export async function fetchCatalogModels(): Promise<any[]> {
  return _fetchPlatform('/api/v1/catalog/models')
}

export async function fetchCatalogChannels(): Promise<any[]> {
  return _fetchPlatform('/api/v1/catalog/channels')
}

// ---- Tunnel usage ----

export async function getTunnelUsage(): Promise<any> {
  const w = await callWails('GetTunnelUsage')
  if (w) return w
  const r = await _httpGet('/api/node/usage?source=tunnel')
  return r ?? { total_requests: 0, total_tokens: 0, model_breakdown: [] }
}

export async function getRecentRequests(limit?: number): Promise<any[]> {
  const w = await callWails('GetRecentRequests', limit ?? 50)
  if (w) return w
  return []
}

// ---- Config ----

export async function getConfig(): Promise<any> {
  const w = await callWails('GetConfig')
  if (w) return w
  return {}
}

export async function saveConfig(cfg: any): Promise<string> {
  const w = await callWails('SaveConfig', cfg)
  if (w) return w
  // Dev mode fallback
  const r = await _httpPost('/api/node/config', cfg)
  if (r?.status === 'ok') return 'ok'
  return 'error: Wails runtime required for saving config'
}

export async function getPlatformURL(): Promise<string> {
  const r = await callWails('GetPlatformURL')
  return r ?? 'http://localhost:8080'
}

