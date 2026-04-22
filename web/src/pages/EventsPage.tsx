import { useState, useMemo } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Search, ChevronDown, Activity } from 'lucide-react'
import type { EventType, EventRecord } from '../lib/mock-data'
import { events, eventTypes, timeRanges } from '../lib/mock-data'
import { EmptyState } from '../components/EmptyState'

/* ---------- Event type color mapping ---------- */

const EVENT_TYPE_STYLES: Record<string, string> = {
  'task.started': 'text-[var(--color-info)]',
  'task.llm_call': 'text-[var(--color-accent)]',
  'task.tool_call': 'text-[var(--color-fg-muted)]',
  'task.completed': 'text-[var(--color-success)]',
  'task.failed': 'text-[var(--color-error)]',
  'task.policy_blocked': 'text-[var(--color-error)]',
  'task.timeout': 'text-[var(--color-warning)]',
}

/* ---------- Format helpers ---------- */

function formatTimestamp(date: Date): string {
  return date.toLocaleTimeString('en-US', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    fractionalSecondDigits: 3,
  } as Intl.DateTimeFormatOptions)
}

function formatCost(cost: number | null): string {
  if (cost === null) return ''
  return `$${cost.toFixed(3)}`
}

/* ---------- Filter controls ---------- */

interface FiltersProps {
  taskIdFilter: string
  onTaskIdChange: (v: string) => void
  eventTypeFilter: EventType | 'all'
  onEventTypeChange: (v: EventType | 'all') => void
  timeRange: string
  onTimeRangeChange: (v: string) => void
  onClearFilters: () => void
}

function FiltersBar({
  taskIdFilter,
  onTaskIdChange,
  eventTypeFilter,
  onEventTypeChange,
  timeRange,
  onTimeRangeChange,
  onClearFilters,
}: FiltersProps) {
  return (
    <div className="flex items-center gap-3 mb-4 flex-wrap">
      {/* Task ID filter */}
      <div className="relative">
        <Search
          className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--color-fg-subtle)]"
          strokeWidth={1.5}
          aria-hidden="true"
        />
        <input
          type="text"
          value={taskIdFilter}
          onChange={(e) => onTaskIdChange(e.target.value)}
          placeholder="Filter by task ID..."
          className="pl-8 pr-3 py-1.5 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-elevated)] text-sm text-[var(--color-fg)] font-[family-name:var(--font-mono)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] w-[200px]"
          aria-label="Filter by task ID"
        />
      </div>

      {/* Event type select */}
      <div className="relative">
        <select
          value={eventTypeFilter}
          onChange={(e) => onEventTypeChange(e.target.value as EventType | 'all')}
          className="appearance-none pl-3 pr-8 py-1.5 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-elevated)] text-sm text-[var(--color-fg)] font-[family-name:var(--font-mono)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] cursor-pointer"
          aria-label="Filter by event type"
        >
          <option value="all">All events</option>
          {eventTypes.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>
        <ChevronDown
          className="absolute right-2 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--color-fg-subtle)] pointer-events-none"
          strokeWidth={1.5}
          aria-hidden="true"
        />
      </div>

      {/* Time range select */}
      <div className="relative">
        <select
          value={timeRange}
          onChange={(e) => onTimeRangeChange(e.target.value)}
          className="appearance-none pl-3 pr-8 py-1.5 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-elevated)] text-sm text-[var(--color-fg)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] cursor-pointer"
          aria-label="Filter by time range"
        >
          {timeRanges.map((r) => (
            <option key={r.value} value={r.value}>
              {r.label}
            </option>
          ))}
        </select>
        <ChevronDown
          className="absolute right-2 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--color-fg-subtle)] pointer-events-none"
          strokeWidth={1.5}
          aria-hidden="true"
        />
      </div>

      {/* Clear filters */}
      {(taskIdFilter || eventTypeFilter !== 'all') && (
        <button
          onClick={onClearFilters}
          className="px-3 py-1.5 rounded-[var(--radius-md)] text-xs font-medium text-[var(--color-fg-muted)] hover:bg-[var(--color-bg-subtle)] hover:text-[var(--color-fg)] transition-colors duration-150"
        >
          Clear filters
        </button>
      )}
    </div>
  )
}

/* ---------- Event row ---------- */

interface EventRowProps {
  event: EventRecord
  isSelected: boolean
  onSelect: (id: string) => void
  onFilterByTask: (taskId: string) => void
}

function EventRow({ event, isSelected, onSelect, onFilterByTask }: EventRowProps) {
  return (
    <tr
      role="row"
      aria-selected={isSelected}
      className={`border-b border-[var(--color-border)] transition-colors duration-150 ${
        isSelected
          ? 'bg-[var(--color-bg-subtle)] border-l-2 border-l-[var(--color-accent)]'
          : 'border-l-2 border-l-transparent hover:bg-[var(--color-bg-subtle)]'
      }`}
      style={{ height: 28 }}
      onClick={() => onSelect(event.id)}
    >
      <td className="px-3 font-[family-name:var(--font-mono)] text-xs text-[var(--color-fg-muted)] tabular whitespace-nowrap">
        {formatTimestamp(event.timestamp)}
      </td>
      <td className="px-3 font-[family-name:var(--font-mono)] text-xs tabular whitespace-nowrap">
        <button
          className="text-[var(--color-fg)] hover:text-[var(--color-accent)] hover:underline cursor-pointer bg-transparent border-none p-0 font-[family-name:var(--font-mono)] text-xs"
          title={event.taskId}
          onClick={(e) => {
            e.stopPropagation()
            onFilterByTask(event.taskId.substring(0, 6))
          }}
        >
          {event.taskId.substring(0, 6)}&hellip;
        </button>
      </td>
      <td
        className={`px-3 font-[family-name:var(--font-mono)] text-xs whitespace-nowrap ${
          EVENT_TYPE_STYLES[event.eventType] ?? 'text-[var(--color-fg)]'
        }`}
      >
        {event.eventType}
      </td>
      <td className="px-3 font-[family-name:var(--font-mono)] text-xs text-[var(--color-fg-muted)] whitespace-nowrap">
        {event.model ?? ''}
      </td>
      <td className="px-3 font-[family-name:var(--font-mono)] text-xs text-[var(--color-fg)] tabular text-right whitespace-nowrap">
        {formatCost(event.cost)}
      </td>
    </tr>
  )
}

/* ---------- Events page ---------- */

export function EventsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [taskIdFilter, setTaskIdFilter] = useState(
    searchParams.get('task') ?? ''
  )
  const [eventTypeFilter, setEventTypeFilter] = useState<EventType | 'all'>(
    (searchParams.get('type') as EventType) ?? 'all'
  )
  const [timeRange, setTimeRange] = useState(
    searchParams.get('since') ?? '1h'
  )
  const [selectedId, setSelectedId] = useState<string | null>(null)

  // Update URL params when filters change
  const updateParams = (
    task: string,
    type: EventType | 'all',
    since: string
  ) => {
    const params = new URLSearchParams()
    if (task) params.set('task', task)
    if (type !== 'all') params.set('type', type)
    if (since !== '1h') params.set('since', since)
    setSearchParams(params, { replace: true })
  }

  const handleTaskIdChange = (v: string) => {
    setTaskIdFilter(v)
    updateParams(v, eventTypeFilter, timeRange)
  }

  const handleEventTypeChange = (v: EventType | 'all') => {
    setEventTypeFilter(v)
    updateParams(taskIdFilter, v, timeRange)
  }

  const handleTimeRangeChange = (v: string) => {
    setTimeRange(v)
    updateParams(taskIdFilter, eventTypeFilter, v)
  }

  const clearFilters = () => {
    setTaskIdFilter('')
    setEventTypeFilter('all')
    setTimeRange('1h')
    setSearchParams({}, { replace: true })
  }

  const filteredEvents = useMemo(() => {
    let result: EventRecord[] = events

    if (taskIdFilter.trim()) {
      const q = taskIdFilter.toLowerCase()
      result = result.filter((e) => e.taskId.toLowerCase().includes(q))
    }

    if (eventTypeFilter !== 'all') {
      result = result.filter((e) => e.eventType === eventTypeFilter)
    }

    return result
  }, [taskIdFilter, eventTypeFilter])

  return (
    <div>
      <h2 className="text-[var(--text-xl)] font-semibold text-[var(--color-fg)] mb-6">
        Events
      </h2>

      {events.length === 0 ? (
        <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-elevated)]">
          <EmptyState
            icon={<Activity size={20} />}
            title="No events yet"
            message="Events stream here in real-time as tasks execute. Submit a task to see activity."
          />
        </div>
      ) : (
      <>
      <FiltersBar
        taskIdFilter={taskIdFilter}
        onTaskIdChange={handleTaskIdChange}
        eventTypeFilter={eventTypeFilter}
        onEventTypeChange={handleEventTypeChange}
        timeRange={timeRange}
        onTimeRangeChange={handleTimeRangeChange}
        onClearFilters={clearFilters}
      />

      <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-elevated)] overflow-hidden">
        <div className="overflow-auto" style={{ maxHeight: 'calc(100vh - 220px)' }}>
          <table className="w-full text-sm" style={{ borderCollapse: 'collapse' }}>
            <caption className="sr-only">
              Event log showing task execution history
            </caption>
            <thead className="sticky top-0 bg-[var(--color-bg-elevated)] z-10">
              <tr className="border-b border-[var(--color-border-strong)]">
                <th
                  className="text-left px-3 py-2 text-xs font-medium text-[var(--color-fg-muted)]"
                  style={{ width: 120 }}
                >
                  Timestamp
                </th>
                <th
                  className="text-left px-3 py-2 text-xs font-medium text-[var(--color-fg-muted)]"
                  style={{ width: 96 }}
                >
                  Task ID
                </th>
                <th
                  className="text-left px-3 py-2 text-xs font-medium text-[var(--color-fg-muted)]"
                  style={{ width: 180 }}
                >
                  Event
                </th>
                <th
                  className="text-left px-3 py-2 text-xs font-medium text-[var(--color-fg-muted)]"
                  style={{ width: 128 }}
                >
                  Model
                </th>
                <th
                  className="text-right px-3 py-2 text-xs font-medium text-[var(--color-fg-muted)]"
                  style={{ width: 80 }}
                >
                  Cost
                </th>
              </tr>
            </thead>
            <tbody>
              {filteredEvents.length === 0 ? (
                <tr>
                  <td
                    colSpan={5}
                    className="text-center py-12 text-sm text-[var(--color-fg-muted)]"
                  >
                    <p>No events match your filters.</p>
                    <button
                      onClick={clearFilters}
                      className="mt-3 px-4 py-2 rounded-[var(--radius-md)] bg-[var(--color-bg-subtle)] text-[var(--color-fg)] border border-[var(--color-border)] text-sm font-medium hover:bg-[var(--color-bg-elevated)]"
                    >
                      Clear filters
                    </button>
                  </td>
                </tr>
              ) : (
                filteredEvents.map((event) => (
                  <EventRow
                    key={event.id}
                    event={event}
                    isSelected={selectedId === event.id}
                    onSelect={setSelectedId}
                    onFilterByTask={(taskId) => handleTaskIdChange(taskId)}
                  />
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Event count */}
      <div className="mt-3 text-xs text-[var(--color-fg-subtle)] font-[family-name:var(--font-mono)] tabular">
        {filteredEvents.length} event{filteredEvents.length !== 1 ? 's' : ''}
      </div>
      </>
      )}
    </div>
  )
}
