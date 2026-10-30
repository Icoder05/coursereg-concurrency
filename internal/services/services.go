package services

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"go-student-service/internal/db"
	"go-student-service/internal/events"
	"go-student-service/internal/models"
	"go-student-service/internal/repositories"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Kind classifies a service error so handlers can pick the HTTP status.
type Kind int

const (
	KindInvalid  Kind = iota + 1 // 400
	KindNotFound                 // 404
	KindConflict                 // 409
)

// Error is a business-rule failure with a stable machine-readable code.
type Error struct {
	Kind    Kind
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func invalid(code, msg string) *Error  { return &Error{KindInvalid, code, msg} }
func notFound(code, msg string) *Error { return &Error{KindNotFound, code, msg} }
func conflict(code, msg string) *Error { return &Error{KindConflict, code, msg} }

// isUniqueViolation reports a Postgres unique-constraint error (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

type Service struct {
	StudentRepo *repositories.StudentRepo //StudentRepo *repositories.StudentRepo: This field holds a pointer (*) to an instance of StudentRepo, which is defined within the repositories package.
	CourseRepo  *repositories.CourseRepo
	ProjectRepo *repositories.ProjectRepo
	Hub         *events.Hub
}

func NewService(hub *events.Hub) *Service {
	return &Service{
		StudentRepo: repositories.NewStudentRepo(),
		CourseRepo:  repositories.NewCourseRepo(),
		ProjectRepo: repositories.NewProjectRepo(),
		Hub:         hub,
	}
}

// record persists an activity row and broadcasts it to live dashboards.
// Called only after the business transaction has committed.
func (s *Service) record(typ, msg string, data map[string]interface{}) {
	a := models.Activity{Type: typ, Message: msg}
	if err := db.DB.Create(&a).Error; err != nil {
		log.Printf("activity insert failed (%s): %v", typ, err)
		a.CreatedAt = time.Now()
	}
	s.Hub.Publish(events.Event{ID: a.ID, Type: typ, Message: msg, Data: data, At: a.CreatedAt})
}

func studentLabel(st *models.Student) string { return fmt.Sprintf("%s (%s)", st.Name, st.StudentID) }

func (s *Service) seatCount(courseID uuid.UUID) int64 {
	var n int64
	db.DB.Model(&models.StudentCourse{}).Where("course_id = ?", courseID).Count(&n)
	return n
}

// ─────────────────────────────────────────────
//
//	CREATE / READ
//
// ─────────────────────────────────────────────

func (s *Service) CreateStudent(st *models.Student) error {
	st.StudentID, st.Name = strings.TrimSpace(st.StudentID), strings.TrimSpace(st.Name)
	if st.StudentID == "" || st.Name == "" {
		return invalid("invalid_payload", "student_id and name required")
	}
	if st.MaxCredits <= 0 {
		st.MaxCredits = 24
	}
	if st.Year <= 0 {
		st.Year = 1
	}
	if err := s.StudentRepo.Create(st); err != nil {
		if isUniqueViolation(err) {
			return conflict("student_exists", fmt.Sprintf("student %s already exists", st.StudentID))
		}
		return err
	}
	s.record("student.created", fmt.Sprintf("New student %s registered", studentLabel(st)),
		map[string]interface{}{"student_id": st.StudentID})
	return nil
}

func (s *Service) CreateCourse(c *models.Course) error {
	c.CourseID, c.Name = strings.TrimSpace(c.CourseID), strings.TrimSpace(c.Name)
	if c.CourseID == "" || c.Name == "" {
		return invalid("invalid_payload", "course_id and name required")
	}
	if c.Capacity < 0 || c.Credits < 0 {
		return invalid("invalid_payload", "capacity and credits must be positive")
	}
	if c.Capacity == 0 {
		c.Capacity = 30
	}
	if c.Credits == 0 {
		c.Credits = 3
	}
	if err := s.CourseRepo.Create(c); err != nil {
		if isUniqueViolation(err) {
			return conflict("course_exists", fmt.Sprintf("course %s already exists", c.CourseID))
		}
		return err
	}
	s.record("course.created", fmt.Sprintf("Course %s %s opened with %d seats", c.CourseID, c.Name, c.Capacity),
		map[string]interface{}{"course_id": c.CourseID})
	return nil
}

func (s *Service) CreateProject(p *models.Project) error {
	p.ProjectID, p.Name = strings.TrimSpace(p.ProjectID), strings.TrimSpace(p.Name)
	if p.ProjectID == "" || p.Name == "" {
		return invalid("invalid_payload", "project_id and name required")
	}
	p.StudentID = nil // assignment goes through AssignProject so the rules apply
	if err := s.ProjectRepo.Create(p); err != nil {
		if isUniqueViolation(err) {
			return conflict("project_exists", fmt.Sprintf("project %s already exists", p.ProjectID))
		}
		return err
	}
	s.record("project.created", fmt.Sprintf("Project %s %q proposed", p.ProjectID, p.Name),
		map[string]interface{}{"project_id": p.ProjectID})
	return nil
}

func (s *Service) GetStudent(ref string) (*models.Student, error) {
	st, err := s.StudentRepo.Resolve(db.DB, ref)
	if err != nil {
		return nil, s.lookupErr(err, "student_not_found", "student "+ref+" not found")
	}
	return st, nil
}

func (s *Service) GetCourse(ref string) (*repositories.CourseSummary, error) {
	c, err := s.CourseRepo.Resolve(db.DB, ref)
	if err != nil {
		return nil, s.lookupErr(err, "course_not_found", "course "+ref+" not found")
	}
	return s.CourseRepo.Summary(c.ID)
}

func (s *Service) GetProject(ref string) (*models.Project, error) {
	p, err := s.ProjectRepo.Resolve(db.DB, ref)
	if err != nil {
		return nil, s.lookupErr(err, "project_not_found", "project "+ref+" not found")
	}
	return p, nil
}

func (s *Service) CourseRoster(ref string) ([]repositories.RosterEntry, error) {
	c, err := s.CourseRepo.Resolve(db.DB, ref)
	if err != nil {
		return nil, s.lookupErr(err, "course_not_found", "course "+ref+" not found")
	}
	return s.CourseRepo.Roster(c.ID)
}

func (s *Service) ListStudents(q string) ([]repositories.StudentSummary, error) {
	return s.StudentRepo.List(strings.TrimSpace(q))
}
func (s *Service) ListCourses() ([]repositories.CourseSummary, error)   { return s.CourseRepo.List() }
func (s *Service) ListProjects() ([]repositories.ProjectSummary, error) { return s.ProjectRepo.List() }

func (s *Service) lookupErr(err error, code, msg string) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return notFound(code, msg)
	}
	return err
}

// ─────────────────────────────────────────────
//
//	ENROLLMENT  (seat + credit rules, race-safe)
//
// ─────────────────────────────────────────────

// Enroll adds the student to every listed course, all-or-nothing.
// courseRefs may be course codes (C101) or UUIDs.
func (s *Service) Enroll(studentRef string, courseRefs []string) ([]models.Course, error) {
	st, added, err := s.enroll(studentRef, courseRefs)
	if err != nil {
		return nil, err
	}
	for _, c := range added {
		n := s.seatCount(c.ID)
		s.record("enrollment.created",
			fmt.Sprintf("%s enrolled in %s %s · %d/%d seats", studentLabel(st), c.CourseID, c.Name, n, c.Capacity),
			map[string]interface{}{"student_id": st.StudentID, "course_id": c.CourseID, "enrolled": n, "capacity": c.Capacity})
	}
	return added, nil
}

// enroll runs the transaction. Lock order is always student row, then course rows
// sorted by id, so concurrent requests cannot deadlock.
func (s *Service) enroll(studentRef string, courseRefs []string) (*models.Student, []models.Course, error) {
	if len(courseRefs) == 0 {
		return nil, nil, invalid("invalid_payload", "course_ids required")
	}
	var st *models.Student
	var added []models.Course

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		st, err = s.StudentRepo.Resolve(tx.Clauses(clause.Locking{Strength: "UPDATE"}), studentRef)
		if err != nil {
			return s.lookupErr(err, "student_not_found", "student "+studentRef+" not found")
		}

		// Validate every course exists before touching anything.
		var courses []models.Course
		var missing []string
		seen := map[uuid.UUID]bool{}
		for _, ref := range courseRefs {
			c, err := s.CourseRepo.Resolve(tx, strings.TrimSpace(ref))
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					missing = append(missing, ref)
					continue
				}
				return err
			}
			if !seen[c.ID] {
				seen[c.ID] = true
				courses = append(courses, *c)
			}
		}
		if len(missing) > 0 {
			return notFound("course_not_found", "courses not found: "+strings.Join(missing, ", "))
		}
		sort.Slice(courses, func(i, j int) bool { return courses[i].ID.String() < courses[j].ID.String() })

		credits, err := s.StudentRepo.CreditsUsed(tx, st.ID)
		if err != nil {
			return err
		}
		for _, c := range courses {
			var already int64
			if err := tx.Model(&models.StudentCourse{}).
				Where("student_id = ? AND course_id = ?", st.ID, c.ID).Count(&already).Error; err != nil {
				return err
			}
			if already > 0 {
				continue // idempotent: re-enrolling is a no-op
			}
			enrolled, err := s.CourseRepo.LockAndCount(tx, c.ID)
			if err != nil {
				return err
			}
			if enrolled >= int64(c.Capacity) {
				return conflict("course_full", fmt.Sprintf("%s is full (%d/%d seats)", c.CourseID, enrolled, c.Capacity))
			}
			if credits+int64(c.Credits) > int64(st.MaxCredits) {
				return conflict("credit_limit", fmt.Sprintf("enrolling in %s would exceed the %d-credit limit (%d used)",
					c.CourseID, st.MaxCredits, credits))
			}
			if err := tx.Create(&models.StudentCourse{StudentID: st.ID, CourseID: c.ID}).Error; err != nil {
				if isUniqueViolation(err) {
					continue
				}
				return err
			}
			if err := tx.Where("student_id = ? AND course_id = ?", st.ID, c.ID).Delete(&models.WaitlistEntry{}).Error; err != nil {
				return err
			}
			credits += int64(c.Credits)
			added = append(added, c)
		}
		return nil
	})
	return st, added, err
}

// Drop removes an enrollment and promotes the next eligible waitlisted student.
func (s *Service) Drop(studentRef, courseRef string) error {
	var st *models.Student
	var c *models.Course
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		st, err = s.StudentRepo.Resolve(tx.Clauses(clause.Locking{Strength: "UPDATE"}), studentRef)
		if err != nil {
			return s.lookupErr(err, "student_not_found", "student "+studentRef+" not found")
		}
		c, err = s.CourseRepo.Resolve(tx, courseRef)
		if err != nil {
			return s.lookupErr(err, "course_not_found", "course "+courseRef+" not found")
		}
		if _, err := s.CourseRepo.LockAndCount(tx, c.ID); err != nil {
			return err
		}
		res := tx.Where("student_id = ? AND course_id = ?", st.ID, c.ID).Delete(&models.StudentCourse{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return notFound("not_enrolled", fmt.Sprintf("%s is not enrolled in %s", st.StudentID, c.CourseID))
		}
		return nil
	})
	if err != nil {
		return err
	}
	n := s.seatCount(c.ID)
	s.record("enrollment.deleted",
		fmt.Sprintf("%s dropped %s %s · %d/%d seats", studentLabel(st), c.CourseID, c.Name, n, c.Capacity),
		map[string]interface{}{"student_id": st.StudentID, "course_id": c.CourseID, "enrolled": n, "capacity": c.Capacity})
	s.promoteWaitlist(c)
	return nil
}

// promoteWaitlist fills freed seats from the queue in FIFO order, skipping
// students who would exceed their credit limit. Each promotion is its own
// transaction through enroll, so the usual locking rules apply.
func (s *Service) promoteWaitlist(c *models.Course) {
	var queue []models.WaitlistEntry
	if err := db.DB.Where("course_id = ?", c.ID).Order("created_at").Find(&queue).Error; err != nil {
		log.Printf("waitlist load failed for %s: %v", c.CourseID, err)
		return
	}
	for _, w := range queue {
		st, added, err := s.enroll(w.StudentID.String(), []string{c.ID.String()})
		var se *Error
		switch {
		case err == nil && len(added) == 1:
			n := s.seatCount(c.ID)
			s.record("waitlist.promoted",
				fmt.Sprintf("%s promoted from waitlist into %s · %d/%d seats", studentLabel(st), c.CourseID, n, c.Capacity),
				map[string]interface{}{"student_id": st.StudentID, "course_id": c.CourseID, "enrolled": n, "capacity": c.Capacity})
		case err == nil:
			// already enrolled by other means; clear the stale queue entry
			db.DB.Delete(&models.WaitlistEntry{}, "id = ?", w.ID)
		case errors.As(err, &se) && se.Code == "credit_limit":
			continue
		case errors.As(err, &se) && se.Code == "course_full":
			return
		case err != nil:
			log.Printf("waitlist promotion failed for %s: %v", c.CourseID, err)
			return
		}
	}
}

func (s *Service) JoinWaitlist(studentRef, courseRef string) (int64, error) {
	var st *models.Student
	var c *models.Course
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		st, err = s.StudentRepo.Resolve(tx.Clauses(clause.Locking{Strength: "UPDATE"}), studentRef)
		if err != nil {
			return s.lookupErr(err, "student_not_found", "student "+studentRef+" not found")
		}
		c, err = s.CourseRepo.Resolve(tx, courseRef)
		if err != nil {
			return s.lookupErr(err, "course_not_found", "course "+courseRef+" not found")
		}
		var already int64
		tx.Model(&models.StudentCourse{}).Where("student_id = ? AND course_id = ?", st.ID, c.ID).Count(&already)
		if already > 0 {
			return conflict("already_enrolled", fmt.Sprintf("%s is already enrolled in %s", st.StudentID, c.CourseID))
		}
		enrolled, err := s.CourseRepo.LockAndCount(tx, c.ID)
		if err != nil {
			return err
		}
		if enrolled < int64(c.Capacity) {
			return conflict("seats_available", fmt.Sprintf("%s has open seats, enroll directly", c.CourseID))
		}
		if err := tx.Create(&models.WaitlistEntry{StudentID: st.ID, CourseID: c.ID}).Error; err != nil {
			if isUniqueViolation(err) {
				return conflict("already_waitlisted", fmt.Sprintf("%s is already on the %s waitlist", st.StudentID, c.CourseID))
			}
			return err
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	var pos int64
	db.DB.Model(&models.WaitlistEntry{}).Where("course_id = ?", c.ID).Count(&pos)
	s.record("waitlist.joined",
		fmt.Sprintf("%s joined the %s waitlist at #%d", studentLabel(st), c.CourseID, pos),
		map[string]interface{}{"student_id": st.StudentID, "course_id": c.CourseID, "position": pos})
	return pos, nil
}

func (s *Service) LeaveWaitlist(studentRef, courseRef string) error {
	st, err := s.GetStudent(studentRef)
	if err != nil {
		return err
	}
	c, err := s.CourseRepo.Resolve(db.DB, courseRef)
	if err != nil {
		return s.lookupErr(err, "course_not_found", "course "+courseRef+" not found")
	}
	res := db.DB.Where("student_id = ? AND course_id = ?", st.ID, c.ID).Delete(&models.WaitlistEntry{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return notFound("not_waitlisted", fmt.Sprintf("%s is not on the %s waitlist", st.StudentID, c.CourseID))
	}
	s.record("waitlist.left", fmt.Sprintf("%s left the %s waitlist", studentLabel(st), c.CourseID),
		map[string]interface{}{"student_id": st.StudentID, "course_id": c.CourseID})
	return nil
}

// ─────────────────────────────────────────────
//
//	ASSIGN PROJECT TO STUDENT  (atomic)
//
// ─────────────────────────────────────────────

// AssignProject claims an unassigned project with a single conditional UPDATE,
// so two concurrent claims cannot both succeed. The partial unique index on
// projects.student_id rejects a second project for the same student.
func (s *Service) AssignProject(studentRef string, projectRef string) error {
	st, err := s.GetStudent(studentRef)
	if err != nil {
		return err
	}
	p, err := s.ProjectRepo.Resolve(db.DB, projectRef)
	if err != nil {
		return s.lookupErr(err, "project_not_found", "project "+projectRef+" not found")
	}

	res := db.DB.Model(&models.Project{}).
		Where("id = ? AND student_id IS NULL", p.ID).
		Updates(map[string]interface{}{"student_id": st.ID, "updated_at": time.Now()})
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return conflict("student_has_project", fmt.Sprintf("%s already has a project", st.StudentID))
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return conflict("project_taken", fmt.Sprintf("project %s is already assigned", p.ProjectID))
	}
	s.record("project.assigned", fmt.Sprintf("%s took project %s %q", studentLabel(st), p.ProjectID, p.Name),
		map[string]interface{}{"student_id": st.StudentID, "project_id": p.ProjectID})
	return nil
}

func (s *Service) UnassignProject(studentRef string) error {
	st, err := s.GetStudent(studentRef)
	if err != nil {
		return err
	}
	var p models.Project
	if err := db.DB.Where("student_id = ?", st.ID).First(&p).Error; err != nil {
		return s.lookupErr(err, "no_project", fmt.Sprintf("%s has no project", st.StudentID))
	}
	res := db.DB.Model(&models.Project{}).Where("id = ? AND student_id = ?", p.ID, st.ID).
		Updates(map[string]interface{}{"student_id": nil, "updated_at": time.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return notFound("no_project", fmt.Sprintf("%s has no project", st.StudentID))
	}
	s.record("project.unassigned", fmt.Sprintf("%s released project %s %q", studentLabel(st), p.ProjectID, p.Name),
		map[string]interface{}{"student_id": st.StudentID, "project_id": p.ProjectID})
	return nil
}

// ─────────────────────────────────────────────
//
//	STUDENT DETAILS
//
// ─────────────────────────────────────────────

type Details struct {
	Student     *models.Student
	Courses     []models.Course
	Waitlist    []repositories.WaitlistItem
	Project     *models.Project
	CreditsUsed int64
}

// StudentDetailsBy looks a student up by id (UUID), student_id or name.
// Courses come from one join query (no N+1).
func (s *Service) StudentDetailsBy(field string, value string) (*Details, error) {
	var student models.Student
	q := db.DB
	switch field {
	case "id":
		if _, err := uuid.Parse(value); err != nil {
			return nil, notFound("student_not_found", "student "+value+" not found")
		}
		q = q.Where("id = ?", value)
	case "student_id":
		q = q.Where("student_id = ?", value)
	default:
		q = q.Where("name = ?", value)
	}
	if err := q.First(&student).Error; err != nil {
		return nil, s.lookupErr(err, "student_not_found", "student "+value+" not found")
	}

	d := &Details{Student: &student}
	var err error
	if d.Courses, err = s.StudentRepo.Courses(student.ID); err != nil {
		return nil, err
	}
	if d.Waitlist, err = s.StudentRepo.Waitlist(student.ID); err != nil {
		return nil, err
	}
	for _, c := range d.Courses {
		d.CreditsUsed += int64(c.Credits)
	}

	var project models.Project
	if err := db.DB.Where("student_id = ?", student.ID).First(&project).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	} else {
		d.Project = &project
	}
	return d, nil
}

// ─────────────────────────────────────────────
//
//	UNASSIGNED PROJECTS / EMPTY COURSES
//
// ─────────────────────────────────────────────
func (s *Service) UnassignedProjects() ([]models.Project, error) {
	ps := []models.Project{}
	if err := db.DB.Where("student_id IS NULL").Order("project_id").Find(&ps).Error; err != nil {
		return nil, err
	}
	return ps, nil
}

func (s *Service) EmptyCourses() ([]models.Course, error) {
	cs := []models.Course{}
	if err := db.DB.Raw(`
        SELECT c.* FROM courses c
        LEFT JOIN student_courses sc ON c.id = sc.course_id
        WHERE sc.course_id IS NULL
        ORDER BY c.course_id
    `).Scan(&cs).Error; err != nil {
		return nil, err
	}
	return cs, nil
}

// ─────────────────────────────────────────────
//
//	DASHBOARD
//
// ─────────────────────────────────────────────

type DeptStat struct {
	Department string `json:"department"`
	Courses    int    `json:"courses"`
	Capacity   int64  `json:"capacity"`
	Enrolled   int64  `json:"enrolled"`
}

type Stats struct {
	Students           int64      `json:"students"`
	Courses            int        `json:"courses"`
	Projects           int64      `json:"projects"`
	Enrollments        int64      `json:"enrollments"`
	SeatsTotal         int64      `json:"seats_total"`
	FullCourses        int        `json:"full_courses"`
	EmptyCourses       int        `json:"empty_courses"`
	Waitlisted         int64      `json:"waitlisted"`
	AssignedProjects   int64      `json:"assigned_projects"`
	UnassignedProjects int64      `json:"unassigned_projects"`
	Departments        []DeptStat `json:"departments"`
	LiveClients        int        `json:"live_clients"`
}

func (s *Service) Stats() (*Stats, error) {
	courses, err := s.CourseRepo.List()
	if err != nil {
		return nil, err
	}
	st := &Stats{Courses: len(courses), LiveClients: s.Hub.Clients(), Departments: []DeptStat{}}
	depts := map[string]*DeptStat{}
	for _, c := range courses {
		st.Enrollments += c.Enrolled
		st.SeatsTotal += int64(c.Capacity)
		st.Waitlisted += c.Waitlisted
		if c.Enrolled >= int64(c.Capacity) {
			st.FullCourses++
		}
		if c.Enrolled == 0 {
			st.EmptyCourses++
		}
		d := depts[c.Department]
		if d == nil {
			d = &DeptStat{Department: c.Department}
			depts[c.Department] = d
		}
		d.Courses++
		d.Capacity += int64(c.Capacity)
		d.Enrolled += c.Enrolled
	}
	for _, d := range depts {
		st.Departments = append(st.Departments, *d)
	}
	sort.Slice(st.Departments, func(i, j int) bool { return st.Departments[i].Department < st.Departments[j].Department })

	if err := db.DB.Model(&models.Student{}).Count(&st.Students).Error; err != nil {
		return nil, err
	}
	if err := db.DB.Model(&models.Project{}).Count(&st.Projects).Error; err != nil {
		return nil, err
	}
	if err := db.DB.Model(&models.Project{}).Where("student_id IS NOT NULL").Count(&st.AssignedProjects).Error; err != nil {
		return nil, err
	}
	st.UnassignedProjects = st.Projects - st.AssignedProjects
	return st, nil
}

func (s *Service) Activities(limit int) ([]models.Activity, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	out := []models.Activity{}
	err := db.DB.Order("id DESC").Limit(limit).Find(&out).Error
	return out, err
}

// Announce broadcasts a non-domain event (e.g. simulator toggled) and logs it to the feed.
func (s *Service) Announce(typ, msg string, data map[string]interface{}) { s.record(typ, msg, data) }
