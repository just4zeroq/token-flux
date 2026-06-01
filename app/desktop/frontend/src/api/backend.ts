// Wails runtime bridge for the Go backend
// In dev mode, use fetch; in prod, use window.runtime
export const backend = {
  async call(method: string, ...args: any[]): Promise<any> {
    if ((window as any).runtime?.Call) {
      return (window as any).runtime.Call(method, ...args)
    }
    // Fallback: mock for dev without Wails
    console.warn('[backend] runtime not available, method:', method, args)
    return null
  },
  async invoke(obj: Record<string, any>): Promise<any> {
    return this.call(obj.method, ...(obj.args || []))
  }
}
