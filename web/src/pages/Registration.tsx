import { useEffect, useMemo, useState } from 'react'
import { api } from '../api'
import { useLiveData } from '../live'
import { DeptBadge, Empty, Flash, Panel, SeatBar, fillLevel, useAction } from '../components/ui'
import type { StudentSummary } from '../types'

export function Registration({ studentId, setStudentId }: { studentId: string | null; setStudentId: (id: string) => void }) {
  const students = useLiveData(() => api.students(), []).data
  const courses = useLiveData(api.courses).data
  const details = useLiveData(() => (studentId ? api.details(studentId) : Promise.resolve(null)), [studentId])
  const { run, busy } = useAction()
  const [dept, setDept] = useState('All')
  const [q, setQ] = useState('')

  // Default to the first student once the list arrives.
  useEffect(() => {
    if (!studentId && students?.length) setStudentId(students[0].student_id)
  }, [studentId, students, setStudentId])

  const d = details.data
  const enrolled = new Set(d?.courses.map((c) => c.course_id))
  const waitlisted = new Map(d?.waitlist.map((w) => [w.course_id, w.position]))
  const creditsLeft = d ? d.student.max_credits - d.student.credits_used : 0

  const depts = useMemo(
    () => ['All', ...Array.from(new Set((courses ?? []).map((c) => c.department).filter(Boolean))).sort()],
    [courses],
  )
  const catalog = (courses ?? []).filter(
    (c) =>
      (dept === 'All' || c.department === dept) &&
      (!q || `${c.course_id} ${c.name} ${c.instructor}`.toLowerCase().includes(q.toLowerCase())),
  )

  const sid = d?.student.student_id ?? ''
  const act = (key: string, fn: () => Promise<unknown>, ok: string) => run(key, fn, ok).then(() => details.reload())

  return (
    <div className="page">
      <div className="reg-top">
        <StudentPicker students={students ?? []} value={studentId} onChange={setStudentId} />
      </div>

      {!d ? (
        <Empty>{details.error ? details.error.message : 'Pick a student to start registering.'}</Empty>
      ) : (
        <div className="reg-grid">
          <div className="reg-side">
            <section className="panel profile">
              <div className="profile-head">
                <div className="avatar" aria-hidden>
                  {d.student.name
                    .split(' ')
                    .map((p) => p[0])
                    .join('')
                    .slice(0, 2)}
                </div>
                <div>
                  <div className="profile-name">{d.student.name}</div>
                  <div className="muted small">
                    <span className="code">{d.student.student_id}</span> · {d.student.department || 'Undeclared'} · Year {d.student.year}
                  </div>
                </div>
              </div>
              <div className="credit-meter">
                <div className="credit-meter-head">
                  <span>Credits</span>
                  <span className="num">
                    <strong>{d.student.credits_used}</strong> / {d.student.max_credits}
                  </span>
                </div>
                <SeatBar enrolled={d.student.credits_used} capacity={d.student.max_credits} compact hideLabel />
              </div>
              <div className="profile-project">
                <span className="muted small">Final-year project</span>
                {d.project ? (
                  <div className="project-line">
                    <div>
                      <span className="code">{d.project.project_id}</span> {d.project.name}
                      <div className="muted small">{d.project.supervisor}</div>
                    </div>
                    <button
                      className="btn btn-ghost btn-sm"
                      disabled={busy === 'unassign'}
                      onClick={() => act('unassign', () => api.unassign(sid), `Released ${d.project!.project_id}`)}
                    >
                      Release
                    </button>
                  </div>
                ) : (
                  <div className="muted">None yet. Claim one from the Projects board.</div>
                )}
              </div>
            </section>

            <Panel title={`My courses (${d.courses.length})`}>
              {d.courses.length ? (
                <ul className="my-list">
                  {d.courses.map((c) => (
                    <li key={c.id}>
                      <div>
                        <span className="code">{c.course_id}</span> {c.name}
                        <div className="muted small">
                          {c.schedule} · {c.credits} cr
                        </div>
                      </div>
                      <button
                        className="btn btn-ghost btn-sm danger"
                        disabled={busy === `drop-${c.course_id}`}
                        onClick={() => act(`drop-${c.course_id}`, () => api.drop(sid, c.course_id), `Dropped ${c.course_id}`)}
                      >
                        Drop
                      </button>
                    </li>
                  ))}
                </ul>
              ) : (
                <Empty>Not enrolled in anything yet.</Empty>
              )}
            </Panel>

            {d.waitlist.length > 0 && (
              <Panel title={`Waitlist (${d.waitlist.length})`}>
                <ul className="my-list">
                  {d.waitlist.map((w) => (
                    <li key={w.id}>
                      <div>
                        <span className="code">{w.course_id}</span> {w.name}
                        <div className="muted small">Position #{w.position} · promoted automatically when a seat frees up</div>
                      </div>
                      <button
                        className="btn btn-ghost btn-sm"
                        disabled={busy === `leave-${w.course_id}`}
                        onClick={() => act(`leave-${w.course_id}`, () => api.leaveWaitlist(sid, w.course_id), `Left ${w.course_id} waitlist`)}
                      >
                        Leave
                      </button>
                    </li>
                  ))}
                </ul>
              </Panel>
            )}
          </div>

          <Panel
            title="Course catalog"
            className="catalog"
            actions={<input className="input input-sm" placeholder="Search courses…" value={q} onChange={(e) => setQ(e.target.value)} />}
          >
            <div className="chips">
              {depts.map((x) => (
                <button key={x} className={`chip ${dept === x ? 'chip-on' : ''}`} onClick={() => setDept(x)}>
                  {x}
                </button>
              ))}
            </div>
            <div className="catalog-list">
              {catalog.map((c) => {
                const isIn = enrolled.has(c.course_id)
                const pos = waitlisted.get(c.course_id)
                const full = fillLevel(c.enrolled, c.capacity) === 'full'
                const tooMany = !isIn && c.credits > creditsLeft
                const k = `c-${c.course_id}`
                let action
                if (isIn) action = <span className="pill pill-ok">Enrolled</span>
                else if (pos) action = <span className="pill pill-info">Waitlist #{pos}</span>
                else if (full)
                  action = (
                    <button className="btn btn-sm" disabled={busy === k} onClick={() => act(k, () => api.joinWaitlist(sid, c.course_id), `Joined ${c.course_id} waitlist`)}>
                      Join waitlist
                    </button>
                  )
                else if (tooMany)
                  action = (
                    <span className="pill pill-muted" title={`Needs ${c.credits} credits, ${creditsLeft} left`}>
                      Over credit limit
                    </span>
                  )
                else
                  action = (
                    <button className="btn btn-primary btn-sm" disabled={busy === k} onClick={() => act(k, () => api.enroll(sid, c.course_id), `Enrolled in ${c.course_id}`)}>
                      Enroll
                    </button>
                  )
                return (
                  <div key={c.id} className={`catalog-row ${isIn ? 'is-in' : ''}`}>
                    <Flash code={c.course_id} />
                    <div className="catalog-info">
                      <div className="catalog-title">
                        <span className="code">{c.course_id}</span> {c.name} <DeptBadge dept={c.department} />
                      </div>
                      <div className="muted small">
                        {c.instructor} · {c.schedule} · {c.credits} credits
                      </div>
                    </div>
                    <div className="catalog-seats">
                      <SeatBar enrolled={c.enrolled} capacity={c.capacity} />
                      {c.waitlisted > 0 && <span className="muted small">{c.waitlisted} waiting</span>}
                    </div>
                    <div className="catalog-action">{action}</div>
                  </div>
                )
              })}
              {!catalog.length && <Empty>No courses match.</Empty>}
            </div>
          </Panel>
        </div>
      )}
    </div>
  )
}

function StudentPicker({ students, value, onChange }: { students: StudentSummary[]; value: string | null; onChange: (id: string) => void }) {
  const [q, setQ] = useState('')
  const [open, setOpen] = useState(false)
  const current = students.find((s) => s.student_id === value)
  const matches = students
    .filter((s) => !q || `${s.student_id} ${s.name}`.toLowerCase().includes(q.toLowerCase()))
    .slice(0, 30)
  return (
    <div className="picker">
      <label className="muted small" htmlFor="picker">
        Registering as
      </label>
      <input
        id="picker"
        className="input"
        placeholder={current ? `${current.name} (${current.student_id})` : 'Search by name or roll number…'}
        value={q}
        onFocus={() => setOpen(true)}
        onBlur={() => window.setTimeout(() => setOpen(false), 150)}
        onChange={(e) => {
          setQ(e.target.value)
          setOpen(true)
        }}
      />
      {open && (
        <ul className="picker-list">
          {matches.map((s) => (
            <li key={s.id}>
              <button
                onMouseDown={() => {
                  onChange(s.student_id)
                  setQ('')
                  setOpen(false)
                }}
              >
                <span>
                  {s.name} <span className="code">{s.student_id}</span>
                </span>
                <span className="muted small">
                  {s.credits_used}/{s.max_credits} cr
                </span>
              </button>
            </li>
          ))}
          {!matches.length && <li className="muted small picker-none">No students match</li>}
        </ul>
      )}
    </div>
  )
}
