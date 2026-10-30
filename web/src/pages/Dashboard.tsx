import { api } from '../api'
import { useLive, useLiveData } from '../live'
import { ActivityFeed, DeptBadge, Empty, Flash, Panel, SeatBar, StatTile, fillLevel } from '../components/ui'

export function Dashboard({ openCourse }: { openCourse: (code: string) => void }) {
  const { feed } = useLive()
  const stats = useLiveData(api.stats).data
  const courses = useLiveData(api.courses).data
  const projects = useLiveData(api.projects).data

  const pct = stats && stats.seats_total ? Math.round((stats.enrollments / stats.seats_total) * 100) : 0
  const sorted = [...(courses ?? [])].sort((a, b) => b.enrolled / b.capacity - a.enrolled / a.capacity)
  const open = (projects ?? []).filter((p) => !p.student_id)
  const empty = (courses ?? []).filter((c) => c.enrolled === 0)

  return (
    <div className="page">
      <div className="tiles">
        <StatTile label="Students" value={stats?.students ?? '–'} sub={`${stats?.courses ?? 0} courses offered`} />
        <StatTile
          label="Seats filled"
          value={stats ? `${pct}%` : '–'}
          sub={stats ? `${stats.enrollments} of ${stats.seats_total} seats` : undefined}
          tone="accent"
        />
        <StatTile label="Full courses" value={stats?.full_courses ?? '–'} sub={`${stats?.empty_courses ?? 0} with no students`} tone={stats?.full_courses ? 'warn' : undefined} />
        <StatTile label="On waitlists" value={stats?.waitlisted ?? '–'} sub="students queued for a seat" />
        <StatTile
          label="Projects taken"
          value={stats ? `${stats.assigned_projects}/${stats.projects}` : '–'}
          sub={`${stats?.unassigned_projects ?? 0} still open`}
        />
        <StatTile label="Live viewers" value={stats?.live_clients ?? '–'} sub="dashboards connected now" />
      </div>

      <div className="dash-grid">
        <div className="dash-main">
          <Panel title="Course capacity" actions={<span className="muted small">sorted by fill · click a course for its roster</span>}>
            {!courses ? (
              <Empty>Loading…</Empty>
            ) : (
              <div className="course-grid">
                {sorted.map((c) => {
                  const level = fillLevel(c.enrolled, c.capacity)
                  const left = Math.max(0, c.capacity - c.enrolled)
                  return (
                    <button key={c.id} className={`course-card level-${level}`} onClick={() => openCourse(c.course_id)}>
                      <Flash code={c.course_id} />
                      <div className="course-card-top">
                        <span className="code">{c.course_id}</span>
                        <DeptBadge dept={c.department} />
                      </div>
                      <div className="course-card-name">{c.name}</div>
                      <SeatBar enrolled={c.enrolled} capacity={c.capacity} />
                      <div className="course-card-foot">
                        {level === 'full' ? (
                          <span className="pill pill-full">Full{c.waitlisted ? ` · ${c.waitlisted} waiting` : ''}</span>
                        ) : (
                          <span className="muted">
                            {left} seat{left === 1 ? '' : 's'} left
                          </span>
                        )}
                        <span className="muted">{c.credits} cr</span>
                      </div>
                    </button>
                  )
                })}
              </div>
            )}
          </Panel>

          <div className="two-col">
            <Panel title="By department">
              {stats?.departments.length ? (
                <ul className="dept-list">
                  {stats.departments.map((d) => (
                    <li key={d.department}>
                      <div className="dept-row">
                        <DeptBadge dept={d.department} />
                        <span className="muted small">{d.courses} courses</span>
                      </div>
                      <SeatBar enrolled={d.enrolled} capacity={d.capacity} />
                    </li>
                  ))}
                </ul>
              ) : (
                <Empty>No courses yet.</Empty>
              )}
            </Panel>
            <Panel title="Needs attention">
              <h3 className="sub-head">Open projects ({open.length})</h3>
              {open.length ? (
                <ul className="mini-list">
                  {open.slice(0, 6).map((p) => (
                    <li key={p.id}>
                      <Flash code={p.project_id} />
                      <span className="code">{p.project_id}</span> {p.name}
                    </li>
                  ))}
                  {open.length > 6 && <li className="muted">+{open.length - 6} more</li>}
                </ul>
              ) : (
                <p className="muted small">Every project has a student.</p>
              )}
              <h3 className="sub-head">Courses with no students ({empty.length})</h3>
              {empty.length ? (
                <ul className="mini-list">
                  {empty.map((c) => (
                    <li key={c.id}>
                      <span className="code">{c.course_id}</span> {c.name}
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="muted small">Every course has at least one student.</p>
              )}
            </Panel>
          </div>
        </div>

        <Panel title="Live activity" className="feed-panel">
          <ActivityFeed items={feed} />
        </Panel>
      </div>
    </div>
  )
}
