import { useState, useMemo, useEffect, useRef } from 'react'
import { Search, Copy, Check, Monitor, Container, Boxes, BookOpen } from 'lucide-react'
import { skills } from '../lib/mock-data'
import type { Skill, Runtime } from '../lib/mock-data'
import { EmptyState } from '../components/EmptyState'

/* ---------- Runtime icon map ---------- */

const RUNTIME_ICONS: Record<Runtime, typeof Monitor> = {
  local: Monitor,
  docker: Container,
  k8s: Boxes,
}

/* ---------- Relative time ---------- */

function relativeTime(date: Date): string {
  const seconds = Math.floor((Date.now() - date.getTime()) / 1000)
  if (seconds < 60) return `${seconds}s ago`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  return `${hours}h ago`
}

/* ---------- YAML code block ---------- */

function YamlBlock({ yaml }: { yaml: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    navigator.clipboard.writeText(yaml).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  const lines = yaml.split('\n')

  return (
    <div className="relative rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-subtle)] overflow-hidden">
      <button
        onClick={handleCopy}
        className="absolute top-2 right-2 p-1.5 rounded-[var(--radius-sm)] text-[var(--color-fg-muted)] hover:bg-[var(--color-bg-elevated)] hover:text-[var(--color-fg)] transition-opacity duration-150"
        aria-label="Copy YAML to clipboard"
      >
        {copied ? (
          <Check className="w-4 h-4 text-[var(--color-success)]" aria-hidden="true" />
        ) : (
          <Copy className="w-4 h-4" aria-hidden="true" />
        )}
      </button>
      <pre className="p-4 pr-12 overflow-x-auto text-xs leading-relaxed font-[family-name:var(--font-mono)] text-[var(--color-fg)]">
        <code>
          {lines.map((line, i) => (
            <div key={i} className="flex">
              <span className="select-none w-8 shrink-0 text-right pr-3 text-[var(--color-fg-subtle)] tabular">
                {i + 1}
              </span>
              <span>{line}</span>
            </div>
          ))}
        </code>
      </pre>
    </div>
  )
}

/* ---------- Detail pane ---------- */

function SkillDetail({ skill }: { skill: Skill | null }) {
  if (!skill) {
    return (
      <div className="flex items-center justify-center h-full text-sm text-[var(--color-fg-muted)]">
        Select a skill to view details.
      </div>
    )
  }

  const RuntimeIcon = RUNTIME_ICONS[skill.runtime]

  return (
    <div className="overflow-auto h-full p-6" aria-live="polite">
      {/* Title */}
      <h2 className="text-[var(--text-xl)] font-semibold text-[var(--color-fg)]">
        {skill.name}
      </h2>
      <p className="mt-1 text-xs text-[var(--color-fg-muted)] font-[family-name:var(--font-mono)]">
        {skill.version} &middot; loaded from {skill.filename} &middot; last
        reload {relativeTime(skill.lastReload)}
      </p>

      {/* Description */}
      <div className="mt-6">
        <h3 className="text-xs font-medium text-[var(--color-fg-muted)] uppercase tracking-wide mb-2">
          Description
        </h3>
        <p className="text-sm text-[var(--color-fg)]">{skill.description}</p>
      </div>

      {/* Parameters table */}
      {skill.parameters.length > 0 && (
        <div className="mt-6">
          <h3 className="text-xs font-medium text-[var(--color-fg-muted)] uppercase tracking-wide mb-2">
            Parameters
          </h3>
          <table className="w-full text-sm">
            <caption className="sr-only">Skill parameters</caption>
            <thead>
              <tr className="border-b border-[var(--color-border-strong)]">
                <th className="text-left py-2 pr-4 text-xs font-medium text-[var(--color-fg-muted)]">
                  Name
                </th>
                <th className="text-left py-2 pr-4 text-xs font-medium text-[var(--color-fg-muted)]">
                  Type
                </th>
                <th className="text-left py-2 pr-4 text-xs font-medium text-[var(--color-fg-muted)]">
                  Required
                </th>
                <th className="text-left py-2 text-xs font-medium text-[var(--color-fg-muted)]">
                  Description
                </th>
              </tr>
            </thead>
            <tbody>
              {skill.parameters.map((param) => (
                <tr
                  key={param.name}
                  className="border-b border-[var(--color-border)]"
                >
                  <td className="py-2 pr-4 font-[family-name:var(--font-mono)] text-xs text-[var(--color-fg)]">
                    {param.name}
                  </td>
                  <td className="py-2 pr-4 font-[family-name:var(--font-mono)] text-xs text-[var(--color-fg-muted)]">
                    {param.type}
                  </td>
                  <td className="py-2 pr-4 text-xs text-[var(--color-fg-muted)]">
                    {param.required ? 'yes' : 'no'}
                  </td>
                  <td className="py-2 text-xs text-[var(--color-fg-muted)]">
                    {param.description}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Required models */}
      {skill.requiredModels.length > 0 && (
        <div className="mt-6">
          <h3 className="text-xs font-medium text-[var(--color-fg-muted)] uppercase tracking-wide mb-2">
            Required models
          </h3>
          <div className="flex flex-wrap gap-2">
            {skill.requiredModels.map((model) => (
              <span
                key={model}
                className="inline-flex items-center px-2 py-0.5 rounded-[var(--radius-sm)] bg-[var(--color-info-bg)] text-[var(--color-info)] text-xs font-medium font-[family-name:var(--font-mono)]"
              >
                {model}
              </span>
            ))}
          </div>
        </div>
      )}

      {/* Required tools */}
      {skill.requiredTools.length > 0 && (
        <div className="mt-6">
          <h3 className="text-xs font-medium text-[var(--color-fg-muted)] uppercase tracking-wide mb-2">
            Required tools
          </h3>
          <div className="flex flex-wrap gap-2">
            {skill.requiredTools.map((tool) => (
              <span
                key={tool}
                className="inline-flex items-center px-2 py-0.5 rounded-[var(--radius-sm)] bg-[var(--color-bg-subtle)] text-[var(--color-fg)] text-xs font-medium font-[family-name:var(--font-mono)] border border-[var(--color-border)]"
              >
                {tool}
              </span>
            ))}
          </div>
        </div>
      )}

      {/* Runtime */}
      <div className="mt-6">
        <h3 className="text-xs font-medium text-[var(--color-fg-muted)] uppercase tracking-wide mb-2">
          Runtime
        </h3>
        <span className="inline-flex items-center gap-1.5 text-xs font-medium font-[family-name:var(--font-mono)]">
          <RuntimeIcon className="w-4 h-4" strokeWidth={1.5} aria-hidden="true" />
          {skill.runtime}
        </span>
      </div>

      {/* Raw YAML */}
      <div className="mt-6">
        <h3 className="text-xs font-medium text-[var(--color-fg-muted)] uppercase tracking-wide mb-2">
          Raw YAML
        </h3>
        <YamlBlock yaml={skill.yaml} />
      </div>
    </div>
  )
}

/* ---------- Skills page ---------- */

export function SkillsPage() {
  const [selectedName, setSelectedName] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const searchRef = useRef<HTMLInputElement>(null)

  const filteredSkills = useMemo(() => {
    if (!query.trim()) return skills
    const q = query.toLowerCase()
    return skills.filter(
      (s) =>
        s.name.toLowerCase().includes(q) ||
        s.description.toLowerCase().includes(q)
    )
  }, [query])

  const selectedSkill = useMemo(
    () => skills.find((s) => s.name === selectedName) ?? null,
    [selectedName]
  )

  // "/" keyboard shortcut to focus search
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (
        e.key === '/' &&
        document.activeElement?.tagName !== 'INPUT' &&
        document.activeElement?.tagName !== 'TEXTAREA'
      ) {
        e.preventDefault()
        searchRef.current?.focus()
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [])

  // Esc to clear search
  const handleSearchKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      setQuery('')
      searchRef.current?.blur()
    }
  }

  return (
    <div>
      <h2 className="text-[var(--text-xl)] font-semibold text-[var(--color-fg)] mb-6">
        Skills
      </h2>

      {skills.length === 0 ? (
        <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-elevated)]">
          <EmptyState
            icon={<BookOpen size={20} />}
            title="No skills loaded"
            message="Skills are mounted via Helm ConfigMaps. See docs/QUICKSTART.md to add one."
            action={
              <a
                href="https://github.com/MackDing/HermesManager/blob/main/docs/QUICKSTART.md"
                className="text-sm font-medium text-[var(--color-accent)] hover:underline"
              >
                View Quickstart &rarr;
              </a>
            }
          />
        </div>
      ) : (
      <div
        className="flex border border-[var(--color-border)] rounded-[var(--radius-md)] bg-[var(--color-bg-elevated)] overflow-hidden"
        style={{ height: 'calc(100vh - 140px)' }}
      >
        {/* Left pane — skill list */}
        <div className="w-[320px] shrink-0 border-r border-[var(--color-border)] flex flex-col">
          {/* Search */}
          <div className="p-3 border-b border-[var(--color-border)]">
            <div className="relative">
              <Search
                className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--color-fg-subtle)]"
                strokeWidth={1.5}
                aria-hidden="true"
              />
              <input
                ref={searchRef}
                type="text"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={handleSearchKeyDown}
                placeholder="Search by name or description..."
                className="w-full pl-8 pr-3 py-1.5 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg)] text-sm text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)]"
                aria-label="Search skills"
              />
            </div>
          </div>

          {/* Skill list */}
          <div
            className="flex-1 overflow-auto"
            role="listbox"
            aria-label="Skills"
          >
            {filteredSkills.length === 0 ? (
              <div className="p-4 text-sm text-[var(--color-fg-muted)] text-center">
                <p>No skills match &lsquo;{query}&rsquo;.</p>
                <button
                  onClick={() => setQuery('')}
                  className="mt-2 px-3 py-1.5 rounded-[var(--radius-md)] bg-[var(--color-bg-subtle)] text-[var(--color-fg)] border border-[var(--color-border)] text-xs font-medium hover:bg-[var(--color-bg-elevated)]"
                >
                  Clear search
                </button>
              </div>
            ) : (
              filteredSkills.map((skill) => {
                const isSelected = skill.name === selectedName
                return (
                  <div
                    key={skill.name}
                    role="option"
                    aria-selected={isSelected}
                    tabIndex={0}
                    className={`flex items-center justify-between px-4 cursor-pointer text-sm transition-colors duration-150 ${
                      isSelected
                        ? 'bg-[var(--color-bg-subtle)] border-l-2 border-l-[var(--color-accent)]'
                        : 'border-l-2 border-l-transparent hover:bg-[var(--color-bg-subtle)]'
                    }`}
                    style={{ height: 32 }}
                    onClick={() => setSelectedName(skill.name)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') setSelectedName(skill.name)
                    }}
                  >
                    <span
                      className={`font-medium truncate ${
                        isSelected
                          ? 'text-[var(--color-fg)]'
                          : 'text-[var(--color-fg-muted)]'
                      }`}
                    >
                      {skill.name}
                    </span>
                    <span className="text-xs text-[var(--color-fg-subtle)] font-[family-name:var(--font-mono)] ml-2 shrink-0">
                      {skill.version}
                    </span>
                  </div>
                )
              })
            )}
          </div>
        </div>

        {/* Right pane — detail */}
        <div className="flex-1 min-w-0">
          <SkillDetail skill={selectedSkill} />
        </div>
      </div>
      )}
    </div>
  )
}
