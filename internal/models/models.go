package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

//ID               uuid.UUID               `gorm:"type:uuid;primaryKey"              json:"id"`
//└── field name   └────────── field type  └────────────────────────── tag 1         └──── tag 2

type Student struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	StudentID  string    `gorm:"uniqueIndex:idx_students_student_id;size:100;not null" json:"student_id"`
	Name       string    `gorm:"size:255;not null;index:idx_students_name" json:"name"`
	Email      string    `gorm:"size:255" json:"email"`
	Department string    `gorm:"size:100" json:"department"`
	Year       int       `gorm:"not null;default:1" json:"year"`
	MaxCredits int       `gorm:"not null;default:24" json:"max_credits"` // credit ceiling enforced on enrollment
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (s *Student) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

type Course struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	CourseID   string    `gorm:"uniqueIndex:idx_courses_course_id;size:100;not null" json:"course_id"`
	Name       string    `gorm:"size:255;not null" json:"name"`
	Department string    `gorm:"size:100" json:"department"`
	Instructor string    `gorm:"size:255" json:"instructor"`
	Schedule   string    `gorm:"size:100" json:"schedule"` // e.g. "Mon/Wed 10:00"
	Credits    int       `gorm:"not null;default:3" json:"credits"`
	Capacity   int       `gorm:"not null;default:30" json:"capacity"` // seats; enrollment fails with course_full beyond this
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (c *Course) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

type Project struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	ProjectID  string     `gorm:"uniqueIndex:idx_projects_project_id;size:100;not null" json:"project_id"`
	Name       string     `gorm:"size:255;not null" json:"name"`
	Supervisor string     `gorm:"size:255" json:"supervisor"`
	StudentID  *uuid.UUID `gorm:"type:uuid;index:idx_projects_student_id" json:"student_id"` // nullable: unassigned projects
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	// FK projects.student_id -> students.id is created in db.Connect (a GORM tag here
	// is ambiguous because Student also has a StudentID field).
}

func (p *Project) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// StudentCourse is the single definition of the student_courses join table
// (students <-> courses many-to-many). Queries join through it explicitly.
type StudentCourse struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	StudentID uuid.UUID `gorm:"type:uuid;not null;index" json:"student_id"`
	CourseID  uuid.UUID `gorm:"type:uuid;not null;index" json:"course_id"`
	CreatedAt time.Time `json:"created_at"`

	Student Student `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Course  Course  `gorm:"constraint:OnDelete:CASCADE" json:"-"`
}

func (sc *StudentCourse) BeforeCreate(tx *gorm.DB) (err error) {
	if sc.ID == uuid.Nil {
		sc.ID = uuid.New()
	}
	return nil
}

// WaitlistEntry queues a student for a full course; position is ordered by CreatedAt.
type WaitlistEntry struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	StudentID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_waitlist_student_course" json:"student_id"`
	CourseID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_waitlist_student_course;index:idx_waitlist_course" json:"course_id"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`

	Student Student `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Course  Course  `gorm:"constraint:OnDelete:CASCADE" json:"-"`
}

func (w *WaitlistEntry) BeforeCreate(tx *gorm.DB) (err error) {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

// Activity is the persisted feed shown on the dashboard.
type Activity struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Type      string    `gorm:"size:50;not null" json:"type"`
	Message   string    `gorm:"size:500;not null" json:"message"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}
