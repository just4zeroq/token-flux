export const backend = {
  async call(method: string, ...args: any[]): Promise<any> {
    try {
      if ((window as any).runtime?.Call) {
        return (window as any).runtime.Call(method, ...args)
      }
    } catch {}
    console.warn('[backend] runtime not available')
    return null
  },
}
