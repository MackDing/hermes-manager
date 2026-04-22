import { useNavigate } from 'react-router-dom'
import {
  Play,
  CheckCircle2,
  XCircle,
  ShieldAlert,
  TrendingUp,
  TrendingDown,
  Monitor,
  Container,
  Boxes,
  Inbox,
} from 'lucide-react'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip as RechartsTooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
} from 'recharts'
import type { PieLabelRenderProps } from 'recharts'
import { StatusBadge } from '../components/StatusBadge'
import { EmptyState } from '../components/EmptyState'
import type { StatCard as StatCardType, Runtime } from '../lib/mock-data'
import {
  statCards,
  activeTasks,
  chartData,
  runtimeDistribution,
} from '../lib/mock-data'

/* ---------- Stat Card ---------- */

const STAT_ICONS: Record<string, typeof Play> = {
  Running: Play,
  Completed: CheckCircle2,
  Failed: XCircle,
  'Policy Blocked': ShieldAlert,
}

const STAT_ICON_COLORS: Record<string, string> = {
  info: 'text-[var(--color-info)]',
  success: 'text-[var(--color-success)]',
  error: 'text-[var(--color-error)]',
  warning: 'text-[var(--color-warning)]',
}

function StatCard({ card }: { card: StatCardType }) {
  const Icon = STAT_ICONS[card.label] ?? Play
  const TrendIcon = card.trend >= 0 ? TrendingUp : TrendingDown
  const trendColor =
    card.status === 'error' || card.status === 'warning'
      ? card.trend > 0
        ? 'text-[var(--color-error)]'
        : 'text-[var(--color-success)]'
      : card.trend >= 0
        ? 'text-[var(--color-success)]'
        : 'text-[var(--color-error)]'

  return (
    <article className="p-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-elevated)]">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-[var(--color-fg-muted)] uppercase tracking-wide">
          {card.label}
        </span>
        <Icon
          className={`w-4 h-4 ${STAT_ICON_COLORS[card.status]}`}
          strokeWidth={1.5}
          aria-hidden="true"
        />
      </div>
      <div className="mt-2 text-2xl font-semibold text-[var(--color-fg)] font-[family-name:var(--font-mono)] tabular">
        {card.value}
      </div>
      <div className="mt-1 text-xs text-[var(--color-fg-muted)] flex items-center gap-1">
        <TrendIcon className={`w-3 h-3 ${trendColor}`} aria-hidden="true" />
        <span className={trendColor}>
          {card.trend >= 0 ? '+' : ''}
          {card.trend}
        </span>{' '}
        {card.trendLabel}
      </div>
    </article>
  )
}

/* ---------- Runtime badge ---------- */

const RUNTIME_ICONS: Record<Runtime, typeof Monitor> = {
  local: Monitor,
  docker: Container,
  k8s: Boxes,
}

const RUNTIME_COLORS: Record<Runtime, string> = {
  local: 'text-[var(--color-runtime-local)]',
  docker: 'text-[var(--color-runtime-docker)]',
  k8s: 'text-[var(--color-runtime-k8s)]',
}

function RuntimeBadge({ runtime }: { runtime: Runtime }) {
  const Icon = RUNTIME_ICONS[runtime]
  return (
    <span
      className={`inline-flex items-center gap-1 text-xs font-medium font-[family-name:var(--font-mono)] ${RUNTIME_COLORS[runtime]}`}
    >
      <Icon className="w-3 h-3" strokeWidth={1.5} aria-hidden="true" />
      {runtime}
    </span>
  )
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

/* ---------- Donut chart colors ---------- */

const DONUT_COLORS: Record<Runtime, string> = {
  local: '#7C3AED',
  docker: '#0EA5E9',
  k8s: '#2563EB',
}

/* ---------- Custom tooltip for line chart ---------- */

interface LineTooltipProps {
  active?: boolean
  payload?: Array<{ value: number }>
  label?: string
}

function LineChartTooltip({ active, payload, label }: LineTooltipProps) {
  if (!active || !payload?.length) return null
  return (
    <div className="rounded-[var(--radius-md)] px-3 py-2 bg-[var(--color-primary)] text-[var(--color-primary-fg)] text-xs font-[family-name:var(--font-mono)] shadow-[var(--shadow-md)]">
      <div>{label}</div>
      <div className="font-medium tabular">{payload[0].value} tasks/min</div>
    </div>
  )
}

/* ---------- Donut label ---------- */

function renderDonutLabel(props: PieLabelRenderProps) {
  const cx = Number(props.cx ?? 0)
  const cy = Number(props.cy ?? 0)
  const midAngle = Number(props.midAngle ?? 0)
  const innerRadius = Number(props.innerRadius ?? 0)
  const outerRadius = Number(props.outerRadius ?? 0)
  const percent = Number(props.percent ?? 0)

  const RADIAN = Math.PI / 180
  const radius = innerRadius + (outerRadius - innerRadius) * 0.5
  const x = cx + radius * Math.cos(-midAngle * RADIAN)
  const y = cy + radius * Math.sin(-midAngle * RADIAN)

  return (
    <text
      x={x}
      y={y}
      fill="white"
      textAnchor="middle"
      dominantBaseline="central"
      className="text-xs font-medium"
    >
      {`${(percent * 100).toFixed(0)}%`}
    </text>
  )
}

/* ---------- Dashboard page ---------- */

export function DashboardPage() {
  const navigate = useNavigate()

  return (
    <div>
      <h2 className="text-[var(--text-xl)] font-semibold text-[var(--color-fg)] mb-6">
        Dashboard
      </h2>

      {/* Stat cards row */}
      <div className="grid grid-cols-4 gap-4 mb-6">
        {statCards.map((card) => (
          <StatCard key={card.label} card={card} />
        ))}
      </div>

      {/* Line chart */}
      <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-4 mb-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-sm font-medium text-[var(--color-fg)]">
            Tasks per minute (24h)
          </h3>
          <span className="text-xs text-[var(--color-fg-subtle)] font-[family-name:var(--font-mono)]">
            24h
          </span>
        </div>
        <div style={{ height: 240 }}>
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={chartData}>
              <XAxis
                dataKey="hour"
                tick={{ fill: 'var(--color-fg-subtle)', fontSize: 11 }}
                tickLine={false}
                axisLine={{ stroke: 'var(--color-border)' }}
                interval={2}
              />
              <YAxis
                tick={{ fill: 'var(--color-fg-subtle)', fontSize: 11 }}
                tickLine={false}
                axisLine={false}
                width={32}
              />
              <RechartsTooltip content={<LineChartTooltip />} />
              <Line
                type="monotone"
                dataKey="tasksPerMinute"
                stroke="var(--color-accent)"
                strokeWidth={2}
                dot={false}
                activeDot={{
                  r: 5,
                  fill: 'var(--color-primary)',
                  stroke: 'var(--color-accent)',
                  strokeWidth: 2,
                }}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Bottom section: Active tasks + Donut */}
      <div className="grid grid-cols-[1fr_320px] gap-6">
        {/* Active tasks table */}
        <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-elevated)]">
          <div className="flex items-center justify-between px-4 py-3 border-b border-[var(--color-border)]">
            <h3 className="text-sm font-medium text-[var(--color-fg)]">
              Active tasks
            </h3>
            <span className="text-xs text-[var(--color-fg-subtle)] font-[family-name:var(--font-mono)] tabular">
              ({activeTasks.length})
            </span>
          </div>
          {activeTasks.length === 0 ? (
            <EmptyState
              icon={<Inbox size={20} />}
              title="No active tasks"
              message="Tasks appear here once agents start running. See the Quickstart for how to submit one."
              action={
                <a
                  href="https://github.com/MackDing/HermesManager/blob/main/docs/QUICKSTART.md"
                  className="text-sm font-medium text-[var(--color-accent)] hover:underline"
                >
                  View Quickstart &rarr;
                </a>
              }
            />
          ) : (
          <div className="overflow-auto max-h-[320px]">
            <table className="w-full text-sm">
              <caption className="sr-only">
                Active tasks currently running in HermesManager
              </caption>
              <thead className="sticky top-0 bg-[var(--color-bg-elevated)]">
                <tr className="border-b border-[var(--color-border-strong)]">
                  <th className="text-left px-4 py-2 text-xs font-medium text-[var(--color-fg-muted)]">
                    Task ID
                  </th>
                  <th className="text-left px-4 py-2 text-xs font-medium text-[var(--color-fg-muted)]">
                    Skill
                  </th>
                  <th className="text-left px-4 py-2 text-xs font-medium text-[var(--color-fg-muted)]">
                    Runtime
                  </th>
                  <th className="text-left px-4 py-2 text-xs font-medium text-[var(--color-fg-muted)]">
                    Started
                  </th>
                  <th className="text-left px-4 py-2 text-xs font-medium text-[var(--color-fg-muted)]">
                    Status
                  </th>
                </tr>
              </thead>
              <tbody>
                {activeTasks.map((task) => (
                  <tr
                    key={task.taskId}
                    className="border-b border-[var(--color-border)] hover:bg-[var(--color-bg-subtle)] cursor-pointer transition-colors duration-150"
                    onClick={() =>
                      navigate(
                        `/events?task=${task.taskId.substring(0, 8)}`
                      )
                    }
                  >
                    <td className="px-4 py-2 font-[family-name:var(--font-mono)] text-xs tabular">
                      <span title={task.taskId}>
                        {task.taskId.substring(0, 8)}&hellip;
                      </span>
                    </td>
                    <td className="px-4 py-2 text-[var(--color-fg)]">
                      {task.skill}
                    </td>
                    <td className="px-4 py-2">
                      <RuntimeBadge runtime={task.runtime} />
                    </td>
                    <td className="px-4 py-2 font-[family-name:var(--font-mono)] text-xs text-[var(--color-fg-muted)] tabular">
                      {relativeTime(task.startedAt)}
                    </td>
                    <td className="px-4 py-2">
                      <StatusBadge status={task.status} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          )}
        </div>

        {/* Donut chart */}
        <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-4">
          <h3 className="text-sm font-medium text-[var(--color-fg)] mb-4">
            Task distribution by runtime
          </h3>
          <div className="flex justify-center" style={{ height: 180 }}>
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={runtimeDistribution}
                  dataKey="count"
                  nameKey="runtime"
                  cx="50%"
                  cy="50%"
                  innerRadius={45}
                  outerRadius={80}
                  labelLine={false}
                  label={renderDonutLabel}
                >
                  {runtimeDistribution.map((entry) => (
                    <Cell
                      key={entry.runtime}
                      fill={DONUT_COLORS[entry.runtime]}
                    />
                  ))}
                </Pie>
              </PieChart>
            </ResponsiveContainer>
          </div>
          <div className="mt-4 space-y-2">
            {runtimeDistribution.map((entry) => {
              const Icon = RUNTIME_ICONS[entry.runtime]
              return (
                <div
                  key={entry.runtime}
                  className="flex items-center justify-between text-xs"
                >
                  <span className="flex items-center gap-2 text-[var(--color-fg-muted)]">
                    <Icon
                      className={`w-4 h-4 ${RUNTIME_COLORS[entry.runtime]}`}
                      strokeWidth={1.5}
                      aria-hidden="true"
                    />
                    {entry.runtime}
                  </span>
                  <span className="font-[family-name:var(--font-mono)] tabular text-[var(--color-fg)]">
                    {entry.percentage}%
                  </span>
                </div>
              )
            })}
          </div>
        </div>
      </div>
    </div>
  )
}
