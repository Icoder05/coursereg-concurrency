package handlers

import (
	"encoding/json" //encoding/json → used for JSON decode + encode
	"errors"
	"fmt"
	"log"
	"net/http" //net/http → used for HTTP request, response, status codes.
	"strconv"
	"time"

	"go-student-service/internal/events"
	"go-student-service/internal/models"
	"go-student-service/internal/services"
	"go-student-service/internal/simulator"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *services.Service //svc → a pointer to services.Service.(This allows all handler functions to use service methods)
	sim *simulator.Simulator
}

func NewHandler(svc *services.Service, sim *simulator.Simulator) *Handler {
	return &Handler{svc: svc, sim: sim}
}

// By passing a pointer (*http.Request), only the memory address of the request struct is copied
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/students", h.ListStudents)
	r.Post("/students", h.CreateStudent) //When POST request hits /students Execute h.CreateStudent
	r.Get("/students/details", h.StudentDetails)
	r.Get("/students/{id}", h.GetStudent) //{id} accepts the UUID or the student_id code
	r.Post("/students/{id}/courses", h.EnrollCourses)
	r.Delete("/students/{id}/courses/{courseId}", h.DropCourse)
	r.Post("/students/{id}/waitlist", h.JoinWaitlist)
	r.Delete("/students/{id}/waitlist/{courseId}", h.LeaveWaitlist)
	r.Post("/students/{id}/project", h.AssignProject)
	r.Delete("/students/{id}/project", h.UnassignProject)

	r.Get("/courses", h.ListCourses)
	r.Post("/courses", h.CreateCourse)
	r.Get("/courses/empty", h.EmptyCourses)
	r.Get("/courses/{id}", h.GetCourse)
	r.Get("/courses/{id}/students", h.CourseRoster)

	r.Get("/projects", h.ListProjects)
	r.Post("/projects", h.CreateProject)
	r.Get("/projects/unassigned", h.UnassignedProjects)
	r.Get("/projects/{id}", h.GetProject)

	r.Get("/dashboard/stats", h.Stats)
	r.Get("/activity", h.Activity)
	r.Get("/events", h.Events)

	r.Get("/simulator", h.SimulatorStatus)
	r.Post("/simulator/start", h.SimulatorStart)
	r.Post("/simulator/stop", h.SimulatorStop)
}

// ─────────────────────────────────────────────
//
//	STUDENTS
//
// ─────────────────────────────────────────────

func (h *Handler) ListStudents(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListStudents(r.URL.Query().Get("q"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) CreateStudent(w http.ResponseWriter, r *http.Request) { //w → write response, r → incoming request,(h *Handler) → method uses handler struct
	var payload models.Student
	if !decode(w, r, &payload) {
		return
	}
	if err := h.svc.CreateStudent(&payload); err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, payload)
}

func (h *Handler) GetStudent(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.GetStudent(chi.URLParam(r, "id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *Handler) EnrollCourses(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		CourseIDs []string `json:"course_ids"` // course codes (C101) or UUIDs
	}
	if !decode(w, r, &payload) {
		return
	}
	added, err := h.svc.Enroll(chi.URLParam(r, "id"), payload.CourseIDs)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	codes := []string{}
	for _, c := range added {
		codes = append(codes, c.CourseID)
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"status": "enrolled", "added": codes})
}

func (h *Handler) DropCourse(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Drop(chi.URLParam(r, "id"), chi.URLParam(r, "courseId")); err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "dropped"})
}

func (h *Handler) JoinWaitlist(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		CourseID string `json:"course_id"`
	}
	if !decode(w, r, &payload) {
		return
	}
	if payload.CourseID == "" {
		httpError(w, http.StatusBadRequest, "invalid_payload", "course_id required")
		return
	}
	pos, err := h.svc.JoinWaitlist(chi.URLParam(r, "id"), payload.CourseID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"status": "waitlisted", "position": pos})
}

func (h *Handler) LeaveWaitlist(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.LeaveWaitlist(chi.URLParam(r, "id"), chi.URLParam(r, "courseId")); err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (h *Handler) AssignProject(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ProjectID string `json:"project_id"` // project code (P001) or UUID
	}
	if !decode(w, r, &payload) {
		return
	}
	if payload.ProjectID == "" {
		httpError(w, http.StatusBadRequest, "invalid_payload", "project_id required")
		return
	}
	if err := h.svc.AssignProject(chi.URLParam(r, "id"), payload.ProjectID); err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "assigned"})
}

func (h *Handler) UnassignProject(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.UnassignProject(chi.URLParam(r, "id")); err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unassigned"})
}

func (h *Handler) StudentDetails(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var field, value string
	switch {
	case q.Get("id") != "":
		field, value = "id", q.Get("id")
	case q.Get("student_id") != "":
		field, value = "student_id", q.Get("student_id")
	case q.Get("name") != "":
		field, value = "name", q.Get("name")
	default:
		httpError(w, http.StatusBadRequest, "invalid_payload", "student_id or name required")
		return
	}
	d, err := h.svc.StudentDetailsBy(field, value)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	st := d.Student
	// Shape follows the brief ({student, courses, project}); extra fields feed the dashboard.
	resp := map[string]interface{}{
		"student": map[string]interface{}{
			"id": st.ID, "student_id": st.StudentID, "name": st.Name, "email": st.Email,
			"department": st.Department, "year": st.Year,
			"max_credits": st.MaxCredits, "credits_used": d.CreditsUsed,
		},
		"courses":  []map[string]interface{}{},
		"waitlist": d.Waitlist,
		"project":  nil,
	}
	cs := []map[string]interface{}{}
	for _, c := range d.Courses {
		cs = append(cs, map[string]interface{}{
			"id": c.ID, "course_id": c.CourseID, "name": c.Name, "credits": c.Credits,
			"schedule": c.Schedule, "instructor": c.Instructor, "department": c.Department,
		})
	}
	resp["courses"] = cs
	if d.Project != nil {
		resp["project"] = map[string]interface{}{
			"id": d.Project.ID, "project_id": d.Project.ProjectID, "name": d.Project.Name, "supervisor": d.Project.Supervisor,
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// ─────────────────────────────────────────────
//
//	COURSES
//
// ─────────────────────────────────────────────

func (h *Handler) ListCourses(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListCourses()
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) CreateCourse(w http.ResponseWriter, r *http.Request) {
	var payload models.Course
	if !decode(w, r, &payload) {
		return
	}
	if err := h.svc.CreateCourse(&payload); err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, payload)
}

func (h *Handler) GetCourse(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.GetCourse(chi.URLParam(r, "id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *Handler) CourseRoster(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.CourseRoster(chi.URLParam(r, "id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) EmptyCourses(w http.ResponseWriter, r *http.Request) {
	cs, err := h.svc.EmptyCourses()
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, cs)
}

// ─────────────────────────────────────────────
//
//	PROJECTS
//
// ─────────────────────────────────────────────

func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListProjects()
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var payload models.Project
	if !decode(w, r, &payload) {
		return
	}
	if err := h.svc.CreateProject(&payload); err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, payload)
}

func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetProject(chi.URLParam(r, "id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) UnassignedProjects(w http.ResponseWriter, r *http.Request) {
	ps, err := h.svc.UnassignedProjects()
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

// ─────────────────────────────────────────────
//
//	DASHBOARD + LIVE STREAM
//
// ─────────────────────────────────────────────

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.Stats()
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h *Handler) Activity(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.svc.Activities(limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// Events streams every domain event as Server-Sent Events. The browser's
// EventSource reconnects automatically; a comment line every 15s keeps
// proxies from closing an idle connection.
func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpError(w, http.StatusInternalServerError, "streaming_unsupported", "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering

	ch, unsubscribe := h.svc.Hub.Subscribe()
	defer unsubscribe()

	fmt.Fprintf(w, "retry: 3000\n\n")
	writeEvent(w, events.Event{Type: "connected", Message: "live stream connected", At: time.Now()})
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e, open := <-ch:
			if !open {
				return // hub closed (server shutting down)
			}
			writeEvent(w, e)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func writeEvent(w http.ResponseWriter, e events.Event) {
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	if e.ID > 0 {
		fmt.Fprintf(w, "id: %d\n", e.ID)
	}
	fmt.Fprintf(w, "data: %s\n\n", b)
}

func (h *Handler) SimulatorStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"running": h.sim.Running()})
}

func (h *Handler) SimulatorStart(w http.ResponseWriter, r *http.Request) {
	if h.sim.Start() {
		h.svc.Announce("simulator", "Traffic simulator started", map[string]interface{}{"running": true})
	}
	writeJSON(w, http.StatusOK, map[string]bool{"running": true})
}

func (h *Handler) SimulatorStop(w http.ResponseWriter, r *http.Request) {
	if h.sim.Stop() {
		h.svc.Announce("simulator", "Traffic simulator stopped", map[string]interface{}{"running": false})
	}
	writeJSON(w, http.StatusOK, map[string]bool{"running": false})
}

// ─────────────────────────────────────────────
//
//	HELPERS
//
// ─────────────────────────────────────────────

func decode(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		httpError(w, http.StatusBadRequest, "invalid_payload", "invalid JSON")
		return false
	}
	return true
}

// fail maps service errors to HTTP codes; unexpected errors are logged and hidden from clients.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	var se *services.Error
	if errors.As(err, &se) {
		status := http.StatusBadRequest
		switch se.Kind {
		case services.KindNotFound:
			status = http.StatusNotFound
		case services.KindConflict:
			status = http.StatusConflict
		}
		httpError(w, status, se.Code, se.Message)
		return
	}
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	httpError(w, http.StatusInternalServerError, "internal_error", "unexpected server error")
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, status int, errCode, message string) {
	writeJSON(w, status, map[string]string{"error": errCode, "message": message})
}
