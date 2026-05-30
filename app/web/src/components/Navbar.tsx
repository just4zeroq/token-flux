import { useState, useRef, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '../stores/auth'
import { LanguageSwitcher } from './LanguageSwitcher'

export function Navbar() {
  const { t } = useTranslation()
  const { token, user, logout } = useAuthStore()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  const NAV_LINKS = [
    { href: '/', label: t('nav.home') },
    { href: '/models', label: t('nav.models') },
    { href: '/providers', label: t('nav.providers') },
  ]

  useEffect(() => {
    const onDocClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDocClick)
    return () => document.removeEventListener('mousedown', onDocClick)
  }, [])

  const avatarLetter = user?.username?.charAt(0).toUpperCase() || 'U'

  return (
    <header className="sticky top-0 z-50 h-14 border-b border-border bg-white/95 backdrop-blur supports-[backdrop-filter]:bg-white/60">
      <div className="mx-auto flex h-full max-w-7xl items-center justify-between px-4">
        {/* Logo */}
        <a href="/" className="flex items-center gap-2 shrink-0">
          <div className="flex h-7 w-7 items-center justify-center rounded-md bg-primary text-xs font-bold text-primary-foreground">
            AI
          </div>
          <span className="text-sm font-semibold tracking-tight text-foreground">AI Platform</span>
        </a>

        {/* Nav Links */}
        <nav className="hidden items-center gap-8 md:flex">
          {NAV_LINKS.map((link) => (
            <a
              key={link.href}
              href={link.href}
              className="text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
            >
              {link.label}
            </a>
          ))}
        </nav>

        {/* Right */}
        <div className="flex items-center gap-2">
          <LanguageSwitcher />
          <a
            href={token ? '/console' : '/login'}
            className="inline-flex h-8 items-center rounded-md bg-primary px-3 text-xs font-medium text-primary-foreground transition-opacity hover:opacity-90"
          >
            {t('nav.console')}
          </a>
          {token ? (
            <div ref={ref} className="relative">
              <button
                onClick={() => setOpen(!open)}
                className="flex h-8 w-8 items-center justify-center rounded-full bg-primary text-xs font-bold text-primary-foreground transition-opacity hover:opacity-80"
              >
                {avatarLetter}
              </button>
              {open && (
                <div className="absolute right-0 top-full mt-2 w-44 overflow-hidden rounded-lg border border-border bg-white shadow-lg animate-scale-in">
                  <div className="border-b border-border px-3 py-2.5">
                    <p className="text-sm font-medium text-foreground">{user?.username}</p>
                    <p className="text-xs text-muted-foreground truncate">{user?.email}</p>
                  </div>
                  <a
                    href="/console?tab=settings"
                    onClick={() => setOpen(false)}
                    className="flex items-center gap-2 px-3 py-2 text-sm text-foreground transition-colors hover:bg-muted"
                  >
                    {t('nav.profile')}
                  </a>
                  <button
                    onClick={logout}
                    className="flex w-full items-center gap-2 px-3 py-2 text-sm text-destructive transition-colors hover:bg-muted"
                  >
                    {t('nav.logout')}
                  </button>
                </div>
              )}
            </div>
          ) : (
            <a
              href="/login"
              className="inline-flex h-8 items-center rounded-md border border-border bg-card px-3 text-xs font-medium text-foreground transition-colors hover:bg-muted"
            >
              {t('nav.login')}
            </a>
          )}
        </div>
      </div>
    </header>
  )
}
