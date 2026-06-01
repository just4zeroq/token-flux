// Wails v2 runtime bridge for the Go backend
// In dev mode (wails dev), window.runtime is injected automatically.
export const backend = {
  async call(method: string, ...args: any[]): Promise<any> {
    try {
      if ((window as any).runtime?.Call) {
        return (window as any).runtime.Call(method, ...args)
      }
    } catch { /* ignore */ }
    console.warn('[backend] runtime not available, method:', method, args)
    return null
  },
}
