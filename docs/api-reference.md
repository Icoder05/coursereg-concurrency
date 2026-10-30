# API reference

Base URL: `http://localhost:8080` (direct) or `http://localhost:3000/api` (through nginx).

`{id}` accepts the internal UUID **or** the human code (`S123`, `C101`, `P001`).

## Endpoints

| Method | Path | Notes |
|---|---|---|
| GET | `/students?q=` | list with credits used, course count and project |
| POST | `/students` | `{"student_id","name","email","department","year","max_credits"}` |
| GET | `/students/{id}` | |
| GET | `/students/details?student_id=S123` / `?name=Alice` | courses, waitlist, project |
| POST | `/students/{id}/courses` | `{"course_ids":["C101","C102"]}`, all-or-nothing |
| DELETE | `/students/{id}/courses/{courseId}` | drop; promotes from the waitlist |
| POST | `/students/{id}/waitlist` | `{"course_id":"C101"}` (course must be full) |
| DELETE | `/students/{id}/waitlist/{courseId}` | |
| POST | `/students/{id}/project` | `{"project_id":"P001"}` |
| DELETE | `/students/{id}/project` | release the project |
| GET | `/courses` | list with `enrolled` / `waitlisted` |
| POST | `/courses` | `{"course_id","name","department","instructor","schedule","credits","capacity"}` |
| GET | `/courses/{id}` | |
| GET | `/courses/{id}/students` | roster |
| GET | `/courses/empty` | courses with no students (LEFT JOIN) |
| GET | `/projects` | list with the assigned student |
| POST | `/projects` | |
| GET | `/projects/{id}` | |
| GET | `/projects/unassigned` | |
| GET | `/dashboard/stats` | totals, seat fill, departments, connected viewers |
| GET | `/activity?limit=50` | persisted activity feed |
| GET | `/events` | **SSE stream** of `{id,type,message,data,at}` |
| GET / POST | `/simulator`, `/simulator/start`, `/simulator/stop` | demo traffic generator |
| GET | `/healthz` | liveness |

## Errors

JSON body `{"error":"course_full","message":"CS410 is full (30/30 seats)"}` with status 400 / 404 / 409 / 500. Unexpected errors are logged, not sent to clients.

| Code | Status | When |
|---|---|---|
| `course_full` | 409 | no seats left |
| `credit_limit` | 409 | the enrolment would exceed the student's `max_credits` |
| `course_not_found` | 404 | an id in `course_ids` does not exist |
| `seats_available` | 409 | waitlist join on a course that still has seats |
| `already_enrolled` / `already_waitlisted` | 409 | waitlist join when already enrolled or already queued |
| `project_taken` | 409 | another student claimed the project first |
| `student_has_project` | 409 | the student already holds a project |

## Business rules

| Rule | How it is enforced |
|---|---|
| Course seat limit | Course row locked `FOR UPDATE` before counting; full → `409 course_full` |
| Credit limit per student | Student row locked, enrolled credits summed; over → `409 credit_limit` |
| No duplicate enrolment | Unique index `(student_id, course_id)`; re-enrolling is a no-op |
| Waitlist | Only when the course is full; FIFO; on drop the next eligible student is promoted automatically |
| One project per student | Partial unique index on `projects.student_id WHERE NOT NULL` → `409 student_has_project` |
| One student per project | Claim is a single `UPDATE … WHERE student_id IS NULL` → `409 project_taken` |

Locks are always taken in the order student → courses (sorted by id), so concurrent requests cannot deadlock.

## Examples

```bash
curl -X POST localhost:8080/students -d '{"student_id":"S123","name":"Alice"}'
curl -X POST localhost:8080/students/S123/courses -d '{"course_ids":["CS101","MA101"]}'
curl -X POST localhost:8080/students/S123/project -d '{"project_id":"P001"}'
curl "localhost:8080/students/details?student_id=S123"
curl -N localhost:8080/events          # watch live events
```

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` | see `.env.example` | PostgreSQL connection |
| `DB_SSLMODE` | `disable` | set `require` for hosted databases |
| `HTTP_PORT` | `8080` | |
| `SEED_DEMO_DATA` | `true` | seed a random demo college on an empty database |
| `CORS_ORIGINS` | `*` | comma-separated allowed origins |
| `VITE_API_BASE` (web build) | `/api` | API base URL baked into the React build |
