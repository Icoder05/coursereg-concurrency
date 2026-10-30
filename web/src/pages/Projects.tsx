import { useState, type FormEvent } from 'react'
import { api } from '../api'
import { useLiveData } from '../live'
import { Empty, Flash, Modal, Panel, useAction } from '../components/ui'
import type { Project, StudentSummary } from '../types'

export function Projects() {
  const projects = useLiveData(api.projects).data
  const students = useLiveData(() => api.students(), []).data
  const [creating, setCreating] = useState(false)
  const open = (projects ?? []).filter((p) => !p.student_id)
  const taken = (projects ?? []).filter((p) => p.student_id)
  const free = (students ?? []).filter((s) => !s.project_code)

  return (
    <div className="page">
      <div className="board">
        <Panel
          title={
            <>
              Open <span className="count">{open.length}</span>
            </>
          }
          actions={
            <button className="btn btn-primary btn-sm" onClick={() => setCreating(true)}>
              + New project
            </button>
          }
        >
          <div className="cards">
            {open.map((p) => (
              <OpenCard key={p.id} p={p} free={free} />
            ))}
            {!open.length && <Empty>All projects are taken.</Empty>}
          </div>
        </Panel>
        <Panel
          title={
            <>
              Assigned <span className="count">{taken.length}</span>
            </>
          }
        >
          <div className="cards">
            {taken.map((p) => (
              <TakenCard key={p.id} p={p} />
            ))}
            {!taken.length && <Empty>No projects assigned yet.</Empty>}
          </div>
        </Panel>
      </div>
      {creating && <CreateProject onClose={() => setCreating(false)} />}
    </div>
  )
}

function OpenCard({ p, free }: { p: Project; free: StudentSummary[] }) {
  const { run, busy } = useAction()
  const [who, setWho] = useState('')
  // final-year students first: they're the ones who need a capstone
  const options = [...free].sort((a, b) => b.year - a.year || a.name.localeCompare(b.name))
  return (
    <article className="pcard">
      <Flash code={p.project_id} />
      <div className="pcard-head">
        <span className="code">{p.project_id}</span>
        <span className="pill pill-info">Open</span>
      </div>
      <div className="pcard-name">{p.name}</div>
      <div className="muted small">{p.supervisor || 'No supervisor listed'}</div>
      <div className="pcard-assign">
        <select className="input input-sm" value={who} onChange={(e) => setWho(e.target.value)} aria-label={`Assign ${p.project_id} to`}>
          <option value="">Assign to…</option>
          {options.map((s) => (
            <option key={s.id} value={s.student_id}>
              {s.name} ({s.student_id}) · Y{s.year}
            </option>
          ))}
        </select>
        <button
          className="btn btn-primary btn-sm"
          disabled={!who || busy === 'assign'}
          onClick={() => run('assign', () => api.assign(who, p.project_id), `${p.project_id} assigned`).then(() => setWho(''))}
        >
          Assign
        </button>
      </div>
    </article>
  )
}

function TakenCard({ p }: { p: Project }) {
  const { run, busy } = useAction()
  return (
    <article className="pcard pcard-taken">
      <Flash code={p.project_id} />
      <div className="pcard-head">
        <span className="code">{p.project_id}</span>
        <span className="pill pill-ok">Assigned</span>
      </div>
      <div className="pcard-name">{p.name}</div>
      <div className="muted small">{p.supervisor}</div>
      <div className="pcard-student">
        <span>
          {p.student_name} <span className="code">{p.student_code}</span>
        </span>
        <button
          className="btn btn-ghost btn-sm"
          disabled={busy === 'release'}
          onClick={() => run('release', () => api.unassign(p.student_code!), `${p.project_id} released`)}
        >
          Release
        </button>
      </div>
    </article>
  )
}

function CreateProject({ onClose }: { onClose: () => void }) {
  const { run, busy } = useAction()
  const [f, setF] = useState({ project_id: '', name: '', supervisor: '' })
  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (await run('create', () => api.createProject(f), `Project ${f.project_id} created`)) onClose()
  }
  return (
    <Modal title="New project" onClose={onClose}>
      <form className="form" onSubmit={submit}>
        <label>
          Code
          <input className="input" required value={f.project_id} onChange={(e) => setF({ ...f, project_id: e.target.value })} placeholder="P019" />
        </label>
        <label>
          Title
          <input className="input" required value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} />
        </label>
        <label>
          Supervisor
          <input className="input" value={f.supervisor} onChange={(e) => setF({ ...f, supervisor: e.target.value })} />
        </label>
        <div className="form-actions">
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            Cancel
          </button>
          <button className="btn btn-primary" disabled={busy === 'create'}>
            Create project
          </button>
        </div>
      </form>
    </Modal>
  )
}
