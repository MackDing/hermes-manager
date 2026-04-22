import type { ReactNode } from 'react'

export type EmptyStateProps = {
  icon: ReactNode
  title: string
  message: string
  action?: ReactNode
}

export function EmptyState({ icon, title, message, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center gap-4 py-16 px-6 text-center">
      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-[var(--color-bg-subtle)] text-[var(--color-fg-muted)]">
        {icon}
      </div>
      <div className="space-y-1">
        <h2 className="text-lg font-semibold text-[var(--color-fg)]">{title}</h2>
        <p className="max-w-md text-sm text-[var(--color-fg-muted)]">{message}</p>
      </div>
      {action && <div className="mt-2">{action}</div>}
    </div>
  )
}
