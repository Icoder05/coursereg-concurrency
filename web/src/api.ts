import type { Activity, Course, Project, RosterEntry, Stats, StudentDetails, StudentSummary } from './types'

export const API_BASE: string = import.meta.env.VITE_API_BASE ?? '/api'

export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string) {
    super(message)
  }
}

async function req<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(API_BASE + path, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new ApiError(res.status, data.error ?? 'error', data.message ?? res.statusText)
  }
  return data as T
}

const enc = encodeURIComponent

export const api = {
  stats: () => req<Stats>('GET', '/dashboard/stats'),
  activity: (limit = 40) => req<Activity[]>('GET', `/activity?limit=${limit}`),

  courses: () => req<Course[]>('GET', '/courses'),
  roster: (course: string) => req<RosterEntry[]>('GET', `/courses/${enc(course)}/students`),
  createCourse: (c: Partial<Course>) => req<Course>('POST', '/courses', c),

  students: (q = '') => req<StudentSummary[]>('GET', `/students?q=${enc(q)}`),
  details: (studentId: string) => req<StudentDetails>('GET', `/students/details?student_id=${enc(studentId)}`),
  createStudent: (s: Partial<StudentSummary>) => req<StudentSummary>('POST', '/students', s),

  enroll: (student: string, course: string) =>
    req<{ added: string[] }>('POST', `/students/${enc(student)}/courses`, { course_ids: [course] }),
  drop: (student: string, course: string) => req('DELETE', `/students/${enc(student)}/courses/${enc(course)}`),
  joinWaitlist: (student: string, course: string) =>
    req<{ position: number }>('POST', `/students/${enc(student)}/waitlist`, { course_id: course }),
  leaveWaitlist: (student: string, course: string) => req('DELETE', `/students/${enc(student)}/waitlist/${enc(course)}`),

  projects: () => req<Project[]>('GET', '/projects'),
  createProject: (p: Partial<Project>) => req<Project>('POST', '/projects', p),
  assign: (student: string, project: string) =>
    req('POST', `/students/${enc(student)}/project`, { project_id: project }),
  unassign: (student: string) => req('DELETE', `/students/${enc(student)}/project`),

  simulator: () => req<{ running: boolean }>('GET', '/simulator'),
  setSimulator: (on: boolean) => req<{ running: boolean }>('POST', on ? '/simulator/start' : '/simulator/stop'),
}
