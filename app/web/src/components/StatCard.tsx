import type { ReactNode } from 'react'

interface StatCardProps {
  label: string
  value: string | number
  icon?: ReactNode
  trend?: string
  color?: 'primary' | 'accent' | 'default'
}

export function StatCard({ label, value, icon, trend, color = 'default', children }: StatCardProps & { children?: ReactNode }) {
  const borderColor = {
    primary: 'border-l-primary',
    accent: 'border-l-accent',
    default: 'border-l-border',
  }[color]

  return (
    <div className={`rounded-xl border border-border bg-card p-4 border-l-4 ${borderColor}`}>
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">{label}</p>
        {icon && <span className="text-lg text-muted-foreground">{icon}</span>}
      </div>
      <p className="mt-2 text-2xl font-bold tabular-nums">{value}</p>
      {trend && <p className="mt-1 text-xs text-muted-foreground">{trend}</p>}
      {children}
    </div>
  )
}
