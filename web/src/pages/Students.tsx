import { useState, type FormEvent } from 'react'
import { api } from '../api'
import { useLiveData } from '../live'
import { DeptBadge, Flash, Modal, Panel, SeatBar, useAction } from '../components/ui'

export function Students({ openStudent }: { openStudent: (id: string) => void }) {
  const [q, setQ] = useState('')
  const students = useLiveData(() => api.students(q), [q]).data
  const [creating, setCreating] = useState(false)

  return (
    <div className="page">
      <Panel
        title={`Students (${students?.length ?? 0})`}
        actions={
          <>
            <input className="input input-sm" placeholder="Search name or roll no…" value={q} onChange={(e) => setQ(e.target.value)} />
            <button className="btn btn-primary btn-sm" onClick={() => setCreating(true)}>
              + New student
            </button>
          </>
        }
      >
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th>Roll no</th>
                <th>Name</th>
                <th>Dept</th>
                <th className="num">Year</th>
                <th className="num">Courses</th>
                <th>Credits</th>
                <th className="hide-sm">Project</th>
              </tr>
            </thead>
            <tbody>
              {(students ?? []).map((s) => (
                <tr key={s.id} className="clickable" onClick={() => openStudent(s.student_id)} title="Open in Registration">
                  <td className="code">
                    <Flash code={s.student_id} />
                    {s.student_id}
                  </td>
                  <td>
                    {s.name}
                    <div className="muted small">{s.email}</div>
                  </td>
                  <td>
                    <DeptBadge dept={s.department} />
                  </td>
                  <td className="num">{s.year}</td>
                  <td className="num">{s.course_count}</td>
                  <td className="seats-cell">
                    <SeatBar enrolled={s.credits_used} capacity={s.max_credits} compact />
                  </td>
                  <td className="hide-sm">
                    {s.project_code ? (
                      <>
                        <span className="code">{s.project_code}</span> <span className="muted small">{s.project_name}</span>
                      </>
                    ) : (
                      <span className="muted">–</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Panel>
      {creating && <CreateStudent onClose={() => setCreating(false)} />}
    </div>
  )
}

function CreateStudent({ onClose }: { onClose: () => void }) {
  const { run, busy } = useAction()
  const [f, setF] = useState({ student_id: '', name: '', email: '', department: 'CSE', year: 1, max_credits: 24 })
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) =>
    setF({ ...f, [k]: typeof f[k] === 'number' ? Number(e.target.value) : e.target.value })
  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (await run('create', () => api.createStudent(f), `Student ${f.student_id} registered`)) onClose()
  }
  return (
    <Modal title="New student" onClose={onClose}>
      <form className="form" onSubmit={submit}>
        <div className="form-row">
          <label>
            Roll number
            <input className="input" required value={f.student_id} onChange={set('student_id')} placeholder="2026CSE101" />
          </label>
          <label>
            Department
            <input className="input" value={f.department} onChange={set('department')} />
          </label>
        </div>
        <label>
          Full name
          <input className="input" required value={f.name} onChange={set('name')} />
        </label>
        <label>
          Email
          <input className="input" type="email" value={f.email} onChange={set('email')} />
        </label>
        <div className="form-row">
          <label>
            Year
            <input className="input" type="number" min={1} max={5} value={f.year} onChange={set('year')} />
          </label>
          <label>
            Credit limit
            <input className="input" type="number" min={1} max={40} value={f.max_credits} onChange={set('max_credits')} />
          </label>
        </div>
        <div className="form-actions">
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            Cancel
          </button>
          <button className="btn btn-primary" disabled={busy === 'create'}>
            Register student
          </button>
        </div>
      </form>
    </Modal>
  )
}
