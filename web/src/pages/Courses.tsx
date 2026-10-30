import { useState, type FormEvent } from 'react'
import { api } from '../api'
import { useLiveData } from '../live'
import { DeptBadge, Empty, Flash, Modal, Panel, SeatBar, timeAgo, useAction } from '../components/ui'

export function Courses({ selected, setSelected }: { selected: string | null; setSelected: (code: string | null) => void }) {
  const courses = useLiveData(api.courses).data
  const roster = useLiveData(() => (selected ? api.roster(selected) : Promise.resolve([])), [selected]).data
  const [creating, setCreating] = useState(false)
  const course = courses?.find((c) => c.course_id === selected)

  return (
    <div className="page">
      <div className={`split ${selected ? 'split-open' : ''}`}>
        <Panel
          title={`Courses (${courses?.length ?? 0})`}
          actions={
            <button className="btn btn-primary btn-sm" onClick={() => setCreating(true)}>
              + New course
            </button>
          }
        >
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>Code</th>
                  <th>Course</th>
                  <th>Dept</th>
                  <th className="hide-sm">Schedule</th>
                  <th className="num">Cr</th>
                  <th>Seats</th>
                  <th className="num">Queue</th>
                </tr>
              </thead>
              <tbody>
                {(courses ?? []).map((c) => (
                  <tr key={c.id} className={`clickable ${selected === c.course_id ? 'row-on' : ''}`} onClick={() => setSelected(c.course_id)}>
                    <td className="code">
                      <Flash code={c.course_id} />
                      {c.course_id}
                    </td>
                    <td>
                      {c.name}
                      <div className="muted small">{c.instructor}</div>
                    </td>
                    <td>
                      <DeptBadge dept={c.department} />
                    </td>
                    <td className="hide-sm muted">{c.schedule}</td>
                    <td className="num">{c.credits}</td>
                    <td className="seats-cell">
                      <SeatBar enrolled={c.enrolled} capacity={c.capacity} />
                    </td>
                    <td className="num">{c.waitlisted || '–'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Panel>

        {selected && (
          <Panel
            title={course ? `${course.course_id} · ${course.name}` : selected}
            className="roster"
            actions={
              <button className="btn btn-ghost btn-icon" onClick={() => setSelected(null)} aria-label="Close roster">
                ×
              </button>
            }
          >
            {course && (
              <div className="roster-summary">
                <SeatBar enrolled={course.enrolled} capacity={course.capacity} />
                <div className="muted small">
                  {course.instructor} · {course.schedule} · {course.credits} credits · {course.waitlisted} on waitlist
                </div>
              </div>
            )}
            {roster?.length ? (
              <ol className="roster-list">
                {roster.map((r) => (
                  <li key={r.id}>
                    <Flash code={r.student_id} />
                    <span>
                      {r.name} <span className="code">{r.student_id}</span>
                    </span>
                    <span className="muted small">
                      Y{r.year} · {timeAgo(r.enrolled_at)}
                    </span>
                  </li>
                ))}
              </ol>
            ) : (
              <Empty>No students enrolled yet.</Empty>
            )}
          </Panel>
        )}
      </div>
      {creating && <CreateCourse onClose={() => setCreating(false)} />}
    </div>
  )
}

function CreateCourse({ onClose }: { onClose: () => void }) {
  const { run, busy } = useAction()
  const [f, setF] = useState({ course_id: '', name: '', department: 'CSE', instructor: '', schedule: '', credits: 3, capacity: 30 })
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) =>
    setF({ ...f, [k]: typeof f[k] === 'number' ? Number(e.target.value) : e.target.value })
  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (await run('create', () => api.createCourse(f), `Course ${f.course_id} created`)) onClose()
  }
  return (
    <Modal title="New course" onClose={onClose}>
      <form className="form" onSubmit={submit}>
        <div className="form-row">
          <label>
            Code
            <input className="input" required value={f.course_id} onChange={set('course_id')} placeholder="CS420" />
          </label>
          <label>
            Department
            <input className="input" value={f.department} onChange={set('department')} />
          </label>
        </div>
        <label>
          Name
          <input className="input" required value={f.name} onChange={set('name')} placeholder="Distributed Systems" />
        </label>
        <div className="form-row">
          <label>
            Instructor
            <input className="input" value={f.instructor} onChange={set('instructor')} />
          </label>
          <label>
            Schedule
            <input className="input" value={f.schedule} onChange={set('schedule')} placeholder="Tue/Thu 10:00" />
          </label>
        </div>
        <div className="form-row">
          <label>
            Credits
            <input className="input" type="number" min={1} max={6} value={f.credits} onChange={set('credits')} />
          </label>
          <label>
            Seats
            <input className="input" type="number" min={1} value={f.capacity} onChange={set('capacity')} />
          </label>
        </div>
        <div className="form-actions">
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            Cancel
          </button>
          <button className="btn btn-primary" disabled={busy === 'create'}>
            Create course
          </button>
        </div>
      </form>
    </Modal>
  )
}
