package repositories

import (
	"time"

	"go-student-service/internal/db"
	"go-student-service/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────────
//
//	READ MODELS (rows returned by list/aggregate queries)
//
// ─────────────────────────────────────────────

// CourseSummary is a course plus its live seat usage.
type CourseSummary struct {
	models.Course
	Enrolled   int64 `json:"enrolled"`
	Waitlisted int64 `json:"waitlisted"`
}

// StudentSummary is a student plus credit usage and project (one row per student).
type StudentSummary struct {
	models.Student
	CreditsUsed int64   `json:"credits_used"`
	CourseCount int64   `json:"course_count"`
	ProjectCode *string `json:"project_code"`
	ProjectName *string `json:"project_name"`
}

// ProjectSummary is a project plus the assigned student's codes, if any.
type ProjectSummary struct {
	models.Project
	StudentCode *string `json:"student_code"`
	StudentName *string `json:"student_name"`
}

// WaitlistItem is one course a student is queued for, with queue position.
type WaitlistItem struct {
	CourseUUID uuid.UUID `json:"id"`
	CourseID   string    `json:"course_id"`
	Name       string    `json:"name"`
	Credits    int       `json:"credits"`
	Position   int64     `json:"position"`
	CreatedAt  time.Time `json:"created_at"`
}

// RosterEntry is one student enrolled in a course.
type RosterEntry struct {
	ID         uuid.UUID `json:"id"`
	StudentID  string    `json:"student_id"`
	Name       string    `json:"name"`
	Department string    `json:"department"`
	Year       int       `json:"year"`
	EnrolledAt time.Time `json:"enrolled_at"`
}

// isUUID lets every lookup accept either the internal UUID or the human code (S123, C101, P001).
func isUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// ─────────────────────────────────────────────
//
//	STUDENT REPOSITORY
//
// ─────────────────────────────────────────────
type StudentRepo struct{}

func NewStudentRepo() *StudentRepo { return &StudentRepo{} }

func (r *StudentRepo) Create(s *models.Student) error {
	return db.DB.Create(s).Error
}

// By passing a pointer (*http.Request), only the memory address of the request struct is copied
func (r *StudentRepo) GetByID(id string) (*models.Student, error) { //r *StudentRepo is methong belongs to StudentRepo(to use studentRepo struct)
	var s models.Student
	if err := db.DB.Where("id = ?", id).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *StudentRepo) GetByStudentID(studentID string) (*models.Student, error) {
	var s models.Student
	if err := db.DB.Where("student_id = ?", studentID).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// Resolve finds a student by UUID or by student_id code.
func (r *StudentRepo) Resolve(tx *gorm.DB, ref string) (*models.Student, error) {
	var s models.Student
	// pick one condition; chaining two Where calls on a locked session would AND them
	col := "student_id"
	if isUUID(ref) {
		col = "id"
	}
	if err := tx.Where(col+" = ?", ref).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// List returns students with credit usage; q filters by name or student_id.
func (r *StudentRepo) List(q string) ([]StudentSummary, error) {
	out := []StudentSummary{}
	like := "%" + q + "%"
	err := db.DB.Raw(`
		SELECT s.*,
		       COALESCE(SUM(c.credits), 0) AS credits_used,
		       COUNT(c.id)                 AS course_count,
		       p.project_id                AS project_code,
		       p.name                      AS project_name
		FROM students s
		LEFT JOIN student_courses sc ON sc.student_id = s.id
		LEFT JOIN courses c          ON c.id = sc.course_id
		LEFT JOIN projects p         ON p.student_id = s.id
		WHERE ? = '' OR s.name ILIKE ? OR s.student_id ILIKE ?
		GROUP BY s.id, p.project_id, p.name
		ORDER BY s.student_id`, q, like, like).Scan(&out).Error
	return out, err
}

// Courses returns the courses a student is enrolled in (single join, no N+1).
func (r *StudentRepo) Courses(studentUUID uuid.UUID) ([]models.Course, error) {
	cs := []models.Course{}
	err := db.DB.Raw(`
		SELECT c.* FROM courses c
		JOIN student_courses sc ON sc.course_id = c.id
		WHERE sc.student_id = ?
		ORDER BY c.course_id`, studentUUID).Scan(&cs).Error
	return cs, err
}

func (r *StudentRepo) Waitlist(studentUUID uuid.UUID) ([]WaitlistItem, error) {
	out := []WaitlistItem{}
	err := db.DB.Raw(`
		SELECT c.id AS course_uuid, c.course_id, c.name, c.credits, w.created_at,
		       (SELECT COUNT(*) FROM waitlist_entries w2
		         WHERE w2.course_id = w.course_id AND w2.created_at <= w.created_at) AS position
		FROM waitlist_entries w
		JOIN courses c ON c.id = w.course_id
		WHERE w.student_id = ?
		ORDER BY w.created_at`, studentUUID).Scan(&out).Error
	return out, err
}

// CreditsUsed sums credits of enrolled courses inside the given transaction.
func (r *StudentRepo) CreditsUsed(tx *gorm.DB, studentUUID uuid.UUID) (int64, error) {
	var n int64
	err := tx.Raw(`
		SELECT COALESCE(SUM(c.credits), 0) FROM student_courses sc
		JOIN courses c ON c.id = sc.course_id
		WHERE sc.student_id = ?`, studentUUID).Scan(&n).Error
	return n, err
}

// ─────────────────────────────────────────────
//
//	COURSE REPOSITORY
//
// ─────────────────────────────────────────────
type CourseRepo struct{}

func NewCourseRepo() *CourseRepo { return &CourseRepo{} }

func (r *CourseRepo) Create(c *models.Course) error {
	return db.DB.Create(c).Error
}

func (r *CourseRepo) GetByID(id string) (*models.Course, error) {
	var c models.Course
	if err := db.DB.Where("id = ?", id).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// Only used when course_id = "C01", "C02" etc.
func (r *CourseRepo) GetByCourseID(courseID string) (*models.Course, error) {
	var c models.Course
	if err := db.DB.Where("course_id = ?", courseID).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// Resolve finds a course by UUID or course_id code.
func (r *CourseRepo) Resolve(tx *gorm.DB, ref string) (*models.Course, error) {
	var c models.Course
	// pick one condition; chaining two Where calls on a locked session would AND them
	col := "course_id"
	if isUUID(ref) {
		col = "id"
	}
	if err := tx.Where(col+" = ?", ref).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

const courseSummarySQL = `
	SELECT c.*,
	       (SELECT COUNT(*) FROM student_courses sc WHERE sc.course_id = c.id)  AS enrolled,
	       (SELECT COUNT(*) FROM waitlist_entries w WHERE w.course_id = c.id)   AS waitlisted
	FROM courses c`

func (r *CourseRepo) List() ([]CourseSummary, error) {
	out := []CourseSummary{}
	err := db.DB.Raw(courseSummarySQL + ` ORDER BY c.course_id`).Scan(&out).Error
	return out, err
}

func (r *CourseRepo) Summary(id uuid.UUID) (*CourseSummary, error) {
	var out CourseSummary
	res := db.DB.Raw(courseSummarySQL+` WHERE c.id = ?`, id).Scan(&out)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &out, nil
}

func (r *CourseRepo) Roster(id uuid.UUID) ([]RosterEntry, error) {
	out := []RosterEntry{}
	err := db.DB.Raw(`
		SELECT s.id, s.student_id, s.name, s.department, s.year, sc.created_at AS enrolled_at
		FROM student_courses sc
		JOIN students s ON s.id = sc.student_id
		WHERE sc.course_id = ?
		ORDER BY sc.created_at`, id).Scan(&out).Error
	return out, err
}

// LockAndCount locks the course row (serialising seat checks) and returns current enrollment.
func (r *CourseRepo) LockAndCount(tx *gorm.DB, id uuid.UUID) (int64, error) {
	var locked models.Course
	if err := tx.Raw(`SELECT * FROM courses WHERE id = ? FOR UPDATE`, id).Scan(&locked).Error; err != nil {
		return 0, err
	}
	var n int64
	err := tx.Model(&models.StudentCourse{}).Where("course_id = ?", id).Count(&n).Error
	return n, err
}

// ─────────────────────────────────────────────
//
//	PROJECT REPOSITORY  (UUID FIXED)
//
// ─────────────────────────────────────────────
type ProjectRepo struct{}

func NewProjectRepo() *ProjectRepo { return &ProjectRepo{} }

func (r *ProjectRepo) Create(p *models.Project) error {
	return db.DB.Create(p).Error
}

// CORRECT — lookup by UUID (id column)
func (r *ProjectRepo) GetByID(id string) (*models.Project, error) {
	var p models.Project
	if err := db.DB.Where("id = ?", id).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// Resolve finds a project by UUID or project_id code.
func (r *ProjectRepo) Resolve(tx *gorm.DB, ref string) (*models.Project, error) {
	var p models.Project
	// pick one condition; chaining two Where calls on a locked session would AND them
	col := "project_id"
	if isUUID(ref) {
		col = "id"
	}
	if err := tx.Where(col+" = ?", ref).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProjectRepo) List() ([]ProjectSummary, error) {
	out := []ProjectSummary{}
	err := db.DB.Raw(`
		SELECT p.*, s.student_id AS student_code, s.name AS student_name
		FROM projects p
		LEFT JOIN students s ON s.id = p.student_id
		ORDER BY p.project_id`).Scan(&out).Error
	return out, err
}
