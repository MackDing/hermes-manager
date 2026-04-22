import { Component } from 'react'
import type { ErrorInfo, ReactNode } from 'react'

type Props = { children: ReactNode; fallback?: ReactNode }
type State = { hasError: boolean; error?: Error }

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('[ErrorBoundary]', error, info)
  }

  reset = () => this.setState({ hasError: false, error: undefined })

  render() {
    if (this.state.hasError) {
      return (
        this.props.fallback ?? (
          <div className="flex min-h-[60vh] flex-col items-center justify-center gap-4 px-6 text-center">
            <h2 className="text-xl font-semibold text-[var(--color-fg)]">Something went wrong</h2>
            <p className="max-w-md text-sm text-[var(--color-fg-muted)]">
              {this.state.error?.message ?? 'An unexpected error occurred. Try reloading the page.'}
            </p>
            <button
              onClick={this.reset}
              className="rounded-[var(--radius-md)] border border-[var(--color-border)] px-4 py-2 text-sm font-medium text-[var(--color-fg)] hover:bg-[var(--color-bg-subtle)]"
            >
              Try again
            </button>
          </div>
        )
      )
    }
    return this.props.children
  }
}
