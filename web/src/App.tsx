import { useCallback, useEffect, useState } from 'react'
import { api } from './api'
import { useLive } from './live'
import { useAction } from './components/ui'
import { Dashboard } from './pages/Dashboard'
import { Registration } from './pages/Registration'
import { Courses } from './pages/Courses'
import { Students } from './pages/Students'
import { Projects } from './pages/Projects'

type Page = 'dashboard' | 'registration' | 'courses' | 'students' | 'projects'

const NAV: { id: Page; label: string; icon: string }[] = [
  { id: 'dashboard', label: 'Dashboard', icon: 'M3 13h8V3H3v10zm0 8h8v-6H3v6zm10 0h8V11h-8v10zm0-18v6h8V3h-8z' },
  { id: 'registration', label: 'Registration', icon: 'M19 3h-4.18A3 3 0 0 0 12 1a3 3 0 0 0-2.82 2H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V5a2 2 0 0 0-2-2zm-7 0a1 1 0 1 1 0 2 1 1 0 0 1 0-2zm-2 14-4-4 1.41-1.41L10 14.17l6.59-6.59L18 9l-8 8z' },
  { id: 'courses', label: 'Courses', icon: 'M4 6H2v14a2 2 0 0 0 2 2h14v-2H4V6zm16-4H8a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V4a2 2 0 0 0-2-2zm-1 9H9V9h10v2zm-4 4H9v-2h6v2zm4-8H9V5h10v2z' },
  { id: 'students', label: 'Students', icon: 'M16 11a3 3 0 1 0 0-6 3 3 0 0 0 0 6zm-8 0a3 3 0 1 0 0-6 3 3 0 0 0 0 6zm0 2c-2.33 0-7 1.17-7 3.5V19h14v-2.5C15 14.17 10.33 13 8 13zm8 0c-.29 0-.62.02-.97.05A4.22 4.22 0 0 1 17 16.5V19h6v-2.5c0-2.33-4.67-3.5-7-3.5z' },
  { id: 'projects', label: 'Projects', icon: 'M20 6h-8l-2-2H4a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2zm0 12H4V8h16v10z' },
]

const TITLES: Record<Page, [string, string]> = {
  dashboard: ['Registration overview', 'Seats, waitlists and projects across the college, updating live.'],
  registration: ['Course registration', 'Enroll, drop and join waitlists as a student. Seat counts update as others register.'],
  courses: ['Courses', 'Every offering with live seat usage. Select one to see its roster.'],
  students: ['Students', 'Credit load and project status per student. Select a student to register for them.'],
  projects: ['Final-year projects', 'Claim and release capstone projects. One project per student.'],
}

// Hash routing keeps links shareable without a router dependency: #/page?key=value
function readHash(): { page: Page; param: string | null } {
  const [path, query] = window.location.hash.replace(/^#\/?/, '').split('?')
  const page = (NAV.some((n) => n.id === path) ? path : 'dashboard') as Page
  const param = new URLSearchParams(query ?? '').get('id')
  return { page, param }
}

function rememberedStudent(): string | null {
  try {
    return localStorage.getItem('reg.student')
  } catch {
    return null
  }
}

export function App() {
  const [{ page, param }, setRoute] = useState(readHash)
  const [studentId, setStudentIdState] = useState<string | null>(() => (readHash().page === 'registration' && readHash().param) || rememberedStudent())
  const { connected, simRunning, setSimRunning } = useLive()
  const { run, busy } = useAction()

  useEffect(() => {
    const onHash = () => setRoute(readHash())
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  }, [])

  const go = useCallback((p: Page, id?: string | null) => {
    window.location.hash = `/${p}${id ? `?id=${encodeURIComponent(id)}` : ''}`
  }, [])

  const setStudentId = useCallback(
    (id: string) => {
      setStudentIdState(id)
      try {
        localStorage.setItem('reg.student', id)
      } catch {
        /* storage unavailable */
      }
      if (readHash().page === 'registration') history.replaceState(null, '', `#/registration?id=${encodeURIComponent(id)}`)
    },
    [],
  )

  const toggleSim = () =>
    run('sim', async () => {
      const r = await api.setSimulator(!simRunning)
      setSimRunning(r.running)
    })

  const [title, subtitle] = TITLES[page]

  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark" aria-hidden>
            CR
          </div>
          <div>
            <div className="brand-name">CourseReg</div>
            <div className="brand-sub">Fall 2026 term</div>
          </div>
        </div>
        <nav className="nav">
          {NAV.map((n) => (
            <button key={n.id} className={`nav-item ${page === n.id ? 'nav-on' : ''}`} onClick={() => go(n.id)}>
              <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden>
                <path d={n.icon} fill="currentColor" />
              </svg>
              <span>{n.label}</span>
            </button>
          ))}
        </nav>
        <div className="sidebar-foot">
          <div className={`live ${connected ? 'live-on' : 'live-off'}`}>
            <span className="live-dot" aria-hidden />
            {connected ? 'Live' : 'Reconnecting…'}
          </div>
        </div>
      </aside>

      <main className="main">
        <header className="topbar">
          <div>
            <h1>{title}</h1>
            <p className="muted">{subtitle}</p>
          </div>
          <div className="topbar-actions">
            <div className={`live live-inline ${connected ? 'live-on' : 'live-off'}`}>
              <span className="live-dot" aria-hidden />
              {connected ? 'Live' : 'Offline'}
            </div>
            <button
              className={`btn ${simRunning ? 'btn-warn' : ''}`}
              onClick={toggleSim}
              disabled={busy === 'sim'}
              title="Generates random enrollments, drops, waitlists and project claims through the real API rules"
            >
              {simRunning ? '■ Stop simulator' : '▶ Simulate traffic'}
            </button>
          </div>
        </header>

        {page === 'dashboard' && <Dashboard openCourse={(code) => go('courses', code)} />}
        {page === 'registration' && <Registration studentId={studentId} setStudentId={setStudentId} />}
        {page === 'courses' && <Courses selected={param} setSelected={(code) => go('courses', code)} />}
        {page === 'students' && (
          <Students
            openStudent={(id) => {
              setStudentId(id)
              go('registration', id)
            }}
          />
        )}
        {page === 'projects' && <Projects />}
      </main>
    </div>
  )
}
