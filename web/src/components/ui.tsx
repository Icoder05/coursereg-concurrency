import { createContext, useCallback, useContext, useState, type ReactNode } from 'react'
import { ApiError } from '../api'
import { useLive, useTick, type FeedItem } from '../live'

// ── Toasts ──────────────────────────────────────────────────────────────

type ToastKind = 'ok' | 'err' | 'info'
interface Toast {
  id: number
  kind: ToastKind
  text: string
}
const ToastContext = createContext<(text: string, kind?: ToastKind) => void>(() => {})

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const push = useCallback((text: string, kind: ToastKind = 'ok') => {
    const id = Date.now() + Math.random()
    setToasts((t) => [...t, { id, kind, text }])
    window.setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), 4000)
  }, [])
  return (
    <ToastContext.Provider value={push}>
      {children}
      <div className="toasts" role="status" aria-live="polite">
        {toasts.map((t) => (
          <div key={t.id} className={`toast toast-${t.kind}`}>
            {t.text}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

export const useToast = () => useContext(ToastContext)

/** Runs an API action, toasting success or the server's error message. */
export function useAction() {
  const toast = useToast()
  const [busy, setBusy] = useState<string | null>(null)
  const run = useCallback(
    async (key: string, fn: () => Promise<unknown>, okText?: string) => {
      setBusy(key)
      try {
        await fn()
        if (okText) toast(okText, 'ok')
        return true
      } catch (e) {
        toast(e instanceof ApiError ? e.message : 'Network error, is the service running?', 'err')
        return false
      } finally {
        setBusy(null)
      }
    },
    [toast],
  )
  return { run, busy }
}

// ── Display bits ────────────────────────────────────────────────────────

export function fillLevel(enrolled: number, capacity: number): 'low' | 'mid' | 'high' | 'full' {
  if (capacity <= 0 || enrolled >= capacity) return 'full'
  const r = enrolled / capacity
  return r >= 0.85 ? 'high' : r >= 0.5 ? 'mid' : 'low'
}

export function SeatBar({
  enrolled,
  capacity,
  compact,
  hideLabel,
}: {
  enrolled: number
  capacity: number
  compact?: boolean
  hideLabel?: boolean
}) {
  const pct = capacity > 0 ? Math.min(100, (enrolled / capacity) * 100) : 100
  return (
    <div className={`seatbar ${compact ? 'seatbar-compact' : ''}`}>
      <div className="seatbar-track">
        <div className={`seatbar-fill fill-${fillLevel(enrolled, capacity)}`} style={{ width: `${pct}%` }} />
      </div>
      {!hideLabel && (
        <span className="seatbar-label num">
          {enrolled}/{capacity}
        </span>
      )}
    </div>
  )
}

/** Remounts a short highlight animation whenever `code` changes live. */
export function Flash({ code }: { code: string }) {
  const { touched } = useLive()
  const t = touched[code]
  if (!t || Date.now() - t > 2000) return null
  return <span key={t} className="flash" aria-hidden />
}

export function StatTile({ label, value, sub, tone }: { label: string; value: ReactNode; sub?: ReactNode; tone?: string }) {
  return (
    <div className={`tile ${tone ? `tile-${tone}` : ''}`}>
      <div className="tile-label">{label}</div>
      <div className="tile-value num">{value}</div>
      {sub && <div className="tile-sub">{sub}</div>}
    </div>
  )
}

export function Panel({ title, actions, children, className }: { title: ReactNode; actions?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <section className={`panel ${className ?? ''}`}>
      <header className="panel-head">
        <h2>{title}</h2>
        {actions && <div className="panel-actions">{actions}</div>}
      </header>
      <div className="panel-body">{children}</div>
    </section>
  )
}

export function Empty({ children }: { children: ReactNode }) {
  return <div className="empty">{children}</div>
}

export function DeptBadge({ dept }: { dept: string }) {
  return <span className={`badge dept dept-${dept.toLowerCase() || 'none'}`}>{dept || 'n/a'}</span>
}

export function timeAgo(iso: string): string {
  const s = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 1000))
  if (s < 5) return 'just now'
  if (s < 60) return `${s}s ago`
  if (s < 3600) return `${Math.floor(s / 60)}m ago`
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`
  return `${Math.floor(s / 86400)}d ago`
}

const feedTone: Record<string, string> = {
  'enrollment.created': 'ok',
  'waitlist.promoted': 'ok',
  'project.assigned': 'ok',
  'enrollment.deleted': 'warn',
  'project.unassigned': 'warn',
  'waitlist.joined': 'info',
  'waitlist.left': 'muted',
}

const feedLabel: Record<string, string> = {
  'enrollment.created': 'Enrolled',
  'enrollment.deleted': 'Dropped',
  'waitlist.joined': 'Waitlist',
  'waitlist.left': 'Left queue',
  'waitlist.promoted': 'Promoted',
  'project.assigned': 'Project',
  'project.unassigned': 'Released',
  'student.created': 'New student',
  'course.created': 'New course',
  'project.created': 'New project',
  simulator: 'Simulator',
  system: 'System',
}

export function ActivityFeed({ items, limit = 40 }: { items: FeedItem[]; limit?: number }) {
  useTick(5000)
  if (!items.length) return <Empty>No activity yet. Enroll someone or start the simulator.</Empty>
  return (
    <ol className="feed">
      {items.slice(0, limit).map((a) => (
        <li key={`${a.id}-${a.at}`} className={`feed-item tone-${feedTone[a.type] ?? 'muted'}`}>
          <span className="feed-dot" aria-hidden />
          <div className="feed-main">
            <div className="feed-meta">
              <span className="feed-type">{feedLabel[a.type] ?? a.type}</span>
              <time dateTime={a.at}>{timeAgo(a.at)}</time>
            </div>
            <div className="feed-msg">{a.message}</div>
          </div>
        </li>
      ))}
    </ol>
  )
}

export function Modal({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) {
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" role="dialog" aria-label={title} onClick={(e) => e.stopPropagation()}>
        <header className="modal-head">
          <h2>{title}</h2>
          <button className="btn btn-ghost btn-icon" onClick={onClose} aria-label="Close">
            ×
          </button>
        </header>
        {children}
      </div>
    </div>
  )
}
