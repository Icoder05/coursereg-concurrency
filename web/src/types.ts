export interface Course {
  id: string
  course_id: string
  name: string
  department: string
  instructor: string
  schedule: string
  credits: number
  capacity: number
  enrolled: number
  waitlisted: number
}

export interface StudentSummary {
  id: string
  student_id: string
  name: string
  email: string
  department: string
  year: number
  max_credits: number
  credits_used: number
  course_count: number
  project_code: string | null
  project_name: string | null
}

export interface Project {
  id: string
  project_id: string
  name: string
  supervisor: string
  student_id: string | null
  student_code: string | null
  student_name: string | null
}

export interface DeptStat {
  department: string
  courses: number
  capacity: number
  enrolled: number
}

export interface Stats {
  students: number
  courses: number
  projects: number
  enrollments: number
  seats_total: number
  full_courses: number
  empty_courses: number
  waitlisted: number
  assigned_projects: number
  unassigned_projects: number
  departments: DeptStat[]
  live_clients: number
}

export interface Activity {
  id: number
  type: string
  message: string
  created_at: string
}

export interface LiveEvent {
  id: number
  type: string
  message: string
  data?: Record<string, unknown>
  at: string
}

export interface StudentDetails {
  student: {
    id: string
    student_id: string
    name: string
    email: string
    department: string
    year: number
    max_credits: number
    credits_used: number
  }
  courses: {
    id: string
    course_id: string
    name: string
    credits: number
    schedule: string
    instructor: string
    department: string
  }[]
  waitlist: {
    id: string
    course_id: string
    name: string
    credits: number
    position: number
    created_at: string
  }[]
  project: { id: string; project_id: string; name: string; supervisor: string } | null
}

export interface RosterEntry {
  id: string
  student_id: string
  name: string
  department: string
  year: number
  enrolled_at: string
}
