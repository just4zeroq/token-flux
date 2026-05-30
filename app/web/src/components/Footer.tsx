export function Footer() {
  return (
    <footer className="border-t border-border bg-white">
      <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4 text-xs text-muted-foreground">
        <span>AI Platform</span>
        <span>&copy; {new Date().getFullYear()} AI Platform. All rights reserved.</span>
      </div>
    </footer>
  )
}
