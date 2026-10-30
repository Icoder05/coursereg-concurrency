// Package simulator generates realistic registration traffic (enrolls, drops,
// waitlists, project claims) through the normal service layer, so every action
// obeys the business rules and streams to dashboards like a real user would.
package simulator

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"sync"
	"time"

	"go-student-service/internal/db"
	"go-student-service/internal/models"
	"go-student-service/internal/services"
)

type Simulator struct {
	svc    *services.Service
	mu     sync.Mutex
	cancel context.CancelFunc
	rng    *rand.Rand
}

func New(svc *services.Service) *Simulator {
	return &Simulator{svc: svc, rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

func (s *Simulator) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cancel != nil
}

func (s *Simulator) Start() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return false
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go s.loop(ctx)
	return true
}

func (s *Simulator) Stop() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel == nil {
		return false
	}
	s.cancel()
	s.cancel = nil
	return true
}

func (s *Simulator) loop(ctx context.Context) {
	for {
		// irregular gaps feel more like real users than a fixed ticker
		wait := time.Duration(700+s.rng.Intn(1600)) * time.Millisecond
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
			s.step()
		}
	}
}

func (s *Simulator) step() {
	r := s.rng.Intn(100)
	var err error
	switch {
	case r < 55:
		err = s.enrollRandom()
	case r < 78:
		err = s.dropRandom()
	case r < 88:
		err = s.claimProject()
	case r < 93:
		err = s.releaseProject()
	default:
		err = s.leaveWaitlist()
	}
	var se *services.Error
	if err != nil && !errors.As(err, &se) {
		log.Printf("simulator step failed: %v", err)
	}
}

func (s *Simulator) pickID(query string, args ...interface{}) string {
	var ids []string
	db.DB.Raw(query, args...).Scan(&ids)
	if len(ids) == 0 {
		return ""
	}
	return ids[s.rng.Intn(len(ids))]
}

func (s *Simulator) enrollRandom() error {
	st := s.pickID(`SELECT id::text FROM students`)
	c := s.pickID(`SELECT id::text FROM courses`)
	if st == "" || c == "" {
		return nil
	}
	_, err := s.svc.Enroll(st, []string{c})
	var se *services.Error
	if errors.As(err, &se) && se.Code == "course_full" && s.rng.Intn(2) == 0 {
		_, err = s.svc.JoinWaitlist(st, c)
	}
	return err
}

func (s *Simulator) dropRandom() error {
	var sc models.StudentCourse
	if err := db.DB.Order("random()").First(&sc).Error; err != nil {
		return nil
	}
	return s.svc.Drop(sc.StudentID.String(), sc.CourseID.String())
}

func (s *Simulator) claimProject() error {
	p := s.pickID(`SELECT id::text FROM projects WHERE student_id IS NULL`)
	st := s.pickID(`SELECT s.id::text FROM students s LEFT JOIN projects p ON p.student_id = s.id WHERE p.id IS NULL AND s.year >= 3`)
	if p == "" || st == "" {
		return nil
	}
	return s.svc.AssignProject(st, p)
}

func (s *Simulator) releaseProject() error {
	st := s.pickID(`SELECT student_id::text FROM projects WHERE student_id IS NOT NULL`)
	if st == "" {
		return nil
	}
	return s.svc.UnassignProject(st)
}

func (s *Simulator) leaveWaitlist() error {
	var w models.WaitlistEntry
	if err := db.DB.Order("random()").First(&w).Error; err != nil {
		return s.enrollRandom()
	}
	return s.svc.LeaveWaitlist(w.StudentID.String(), w.CourseID.String())
}
