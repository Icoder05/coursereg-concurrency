import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react'
import { API_BASE, api } from './api'
import type { LiveEvent } from './types'

export interface FeedItem {
  id: number
  type: string
  message: string
  at: string
}

interface LiveState {
  connected: boolean
  /** bumps (debounced) whenever the server reports a change; data hooks refetch on it */
  version: number
  feed: FeedItem[]
  /** course/project code -> time it last changed, for flash highlights */
  touched: Record<string, number>
  simRunning: boolean
  setSimRunning: (on: boolean) => void
}

const LiveContext = createContext<LiveState | null>(null)

export function LiveProvider({ children }: { children: ReactNode }) {
  const [connected, setConnected] = useState(false)
  const [version, setVersion] = useState(0)
  const [feed, setFeed] = useState<FeedItem[]>([])
  const [touched, setTouched] = useState<Record<string, number>>({})
  const [simRunning, setSimRunning] = useState(false)
  const bumpTimer = useRef<number | undefined>(undefined)

  // Coalesce bursts of events into a single refetch.
  const bump = useCallback(() => {
    window.clearTimeout(bumpTimer.current)
    bumpTimer.current = window.setTimeout(() => setVersion((v) => v + 1), 150)
  }, [])

  const loadFeed = useCallback(() => {
    api
      .activity(40)
      .then((rows) => setFeed(rows.map((a) => ({ id: a.id, type: a.type, message: a.message, at: a.created_at }))))
      .catch(() => {})
  }, [])

  useEffect(() => {
    loadFeed()
    api.simulator().then((s) => setSimRunning(s.running)).catch(() => {})

    const es = new EventSource(`${API_BASE}/events`)
    let hadError = false
    es.onopen = () => {
      setConnected(true)
      if (hadError) {
        // missed events while disconnected: resync everything
        loadFeed()
        bump()
      }
    }
    es.onerror = () => {
      hadError = true
      setConnected(false)
    }
    es.onmessage = (msg) => {
      let e: LiveEvent
      try {
        e = JSON.parse(msg.data)
      } catch {
        return
      }
      if (e.type === 'connected') return
      if (e.type === 'simulator') setSimRunning(Boolean(e.data?.running))
      setFeed((f) => [{ id: e.id, type: e.type, message: e.message, at: e.at }, ...f].slice(0, 60))
      const codes = [e.data?.course_id, e.data?.project_id, e.data?.student_id].filter(
        (c): c is string => typeof c === 'string',
      )
      if (codes.length) {
        const now = Date.now()
        setTouched((t) => {
          const next = { ...t }
          codes.forEach((c) => (next[c] = now))
          return next
        })
      }
      bump()
    }
    return () => es.close()
  }, [bump, loadFeed])

  return (
    <LiveContext.Provider value={{ connected, version, feed, touched, simRunning, setSimRunning }}>
      {children}
    </LiveContext.Provider>
  )
}

export function useLive(): LiveState {
  const ctx = useContext(LiveContext)
  if (!ctx) throw new Error('useLive must be used inside LiveProvider')
  return ctx
}

/** Fetches data and refetches whenever a live event arrives (or deps change). */
export function useLiveData<T>(fetcher: () => Promise<T>, deps: unknown[] = []) {
  const { version } = useLive()
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<Error | null>(null)
  const [local, setLocal] = useState(0)

  useEffect(() => {
    let cancelled = false
    fetcher()
      .then((d) => {
        if (!cancelled) {
          setData(d)
          setError(null)
        }
      })
      .catch((e: Error) => !cancelled && setError(e))
    return () => {
      cancelled = true
    }
  }, [version, local, ...deps])

  const reload = useCallback(() => setLocal((n) => n + 1), [])
  return { data, error, reload }
}

/** Re-renders every `ms` so relative timestamps stay fresh. */
export function useTick(ms = 5000) {
  const [, set] = useState(0)
  useEffect(() => {
    const t = window.setInterval(() => set((n) => n + 1), ms)
    return () => window.clearInterval(t)
  }, [ms])
}
