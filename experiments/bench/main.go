// Command bench runs the experiments reported in experiments/README.md.
//
//	RQ1/RQ3  seat over-allocation and latency: naive check-then-insert vs the locked service path
//	RQ2      project lost updates: legacy read-check-save vs the atomic conditional UPDATE
//	RQ4      action-to-screen delay: SSE push vs HTTP polling, against the running service
//
// Every request and trial is written to CSV under -out, with a manifest describing
// the environment, parameters and a hash of the source code.
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-student-service/internal/config"
	"go-student-service/internal/db"
	"go-student-service/internal/events"
	"go-student-service/internal/models"
	"go-student-service/internal/services"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	outDir   = flag.String("out", "experiments/results/latest", "output directory")
	reps     = flag.Int("reps", 30, "measured repetitions per cell")
	warmup   = flag.Int("warmup", 2, "discarded warm-up repetitions per cell")
	seed     = flag.Int64("seed", 20261029, "seed for run order and jitter")
	apiBase  = flag.String("api", "http://service:8080", "running service for RQ4")
	only     = flag.String("only", "rq1,rq2,rq4", "experiments to run")
	poolSize = flag.Int("pool", 60, "max DB connections used by RQ1/RQ2")
	cleanup  = flag.Bool("cleanup", true, "delete benchmark rows afterwards")

	capacity   = 5
	rq1Levels  = []int{10, 25, 50, 100}
	rq2Levels  = []int{2, 5, 10, 20}
	sseLevels  = []int{1, 10, 50, 100}
	pollLevels = []int{500, 2000}
	sseEvents  = 100
	pollEvents = 40
	pollers    = 10
)

var (
	svc      *services.Service
	rng      *rand.Rand
	tag      string
	students []models.Student
	seq      int
)

func main() {
	flag.Parse()
	rng = rand.New(rand.NewSource(*seed))
	tag = "BX" + time.Now().UTC().Format("0102T1504")
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	cfg := config.Load()
	if err := db.Connect(cfg); err != nil {
		log.Fatalf("db: %v", err)
	}
	db.DB.Logger = logger.Default.LogMode(logger.Silent) // expected not-found lookups would flood the log
	sqlDB, _ := db.DB.DB()
	sqlDB.SetMaxOpenConns(*poolSize)
	sqlDB.SetMaxIdleConns(*poolSize)
	svc = services.NewService(events.NewHub()) // private hub: RQ1/RQ2 events stay out of live dashboards

	started := time.Now().UTC()
	makeStudents(100)
	run := strings.Split(*only, ",")
	has := func(s string) bool {
		for _, r := range run {
			if strings.TrimSpace(r) == s {
				return true
			}
		}
		return false
	}
	if has("rq1") {
		runRQ1()
	}
	if has("rq2") {
		runRQ2()
	}
	if has("rq4") {
		runRQ4()
	}
	writeManifest(started, time.Now().UTC())
	if *cleanup {
		cleanupRows()
	}
	log.Printf("done, results in %s", *outDir)
}

// ─────────────────────────────────────────────
//	fixtures
// ─────────────────────────────────────────────

func makeStudents(n int) {
	for i := 1; i <= n; i++ {
		students = append(students, models.Student{
			StudentID: fmt.Sprintf("%s-S%03d", tag, i), Name: fmt.Sprintf("Bench Student %03d", i),
			Department: "BENCH", Year: 4, MaxCredits: 1000,
		})
	}
	must(db.DB.Create(&students).Error)
}

func newCourse(capacity int) models.Course {
	seq++
	c := models.Course{CourseID: fmt.Sprintf("%s-C%04d", tag, seq), Name: "Bench course", Department: "BENCH", Credits: 1, Capacity: capacity}
	must(db.DB.Create(&c).Error)
	return c
}

func newProject() models.Project {
	seq++
	p := models.Project{ProjectID: fmt.Sprintf("%s-P%04d", tag, seq), Name: "Bench project"}
	must(db.DB.Create(&p).Error)
	return p
}

// ─────────────────────────────────────────────
//	RQ1 / RQ3: seat over-allocation and latency
// ─────────────────────────────────────────────

var errFull = errors.New("course_full")

// naiveEnroll mirrors services.Enroll step for step (same queries, same post-commit
// activity write) but takes no row locks, so the arms differ only in locking.
func naiveEnroll(st models.Student, c models.Course) error {
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		s, err := svc.StudentRepo.Resolve(tx, st.ID.String())
		if err != nil {
			return err
		}
		course, err := svc.CourseRepo.Resolve(tx, c.ID.String())
		if err != nil {
			return err
		}
		credits, err := svc.StudentRepo.CreditsUsed(tx, s.ID)
		if err != nil {
			return err
		}
		var already int64
		if err := tx.Model(&models.StudentCourse{}).Where("student_id = ? AND course_id = ?", s.ID, course.ID).Count(&already).Error; err != nil {
			return err
		}
		if already > 0 {
			return nil
		}
		var enrolled int64
		if err := tx.Model(&models.StudentCourse{}).Where("course_id = ?", course.ID).Count(&enrolled).Error; err != nil {
			return err
		}
		if enrolled >= int64(course.Capacity) {
			return errFull
		}
		if credits+int64(course.Credits) > int64(s.MaxCredits) {
			return errors.New("credit_limit")
		}
		if err := tx.Create(&models.StudentCourse{StudentID: s.ID, CourseID: course.ID}).Error; err != nil {
			return err
		}
		return tx.Where("student_id = ? AND course_id = ?", s.ID, course.ID).Delete(&models.WaitlistEntry{}).Error
	})
	if err != nil {
		return err
	}
	var n int64
	db.DB.Model(&models.StudentCourse{}).Where("course_id = ?", c.ID).Count(&n)
	return db.DB.Create(&models.Activity{Type: "enrollment.created",
		Message: fmt.Sprintf("%s (%s) enrolled in %s · %d/%d seats", st.Name, st.StudentID, c.CourseID, n, c.Capacity)}).Error
}

func lockedEnroll(st models.Student, c models.Course) error {
	_, err := svc.Enroll(st.ID.String(), []string{c.ID.String()})
	return err
}

func classify(err error) string {
	var se *services.Error
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, errFull), errors.As(err, &se) && se.Code == "course_full":
		return "full"
	case errors.As(err, &se):
		return "conflict:" + se.Code
	default:
		return "error"
	}
}

type cell struct {
	arm   string
	level int
}

func shuffledCells(arms []string, levels []int) []cell {
	var cs []cell
	for _, a := range arms {
		for _, l := range levels {
			cs = append(cs, cell{a, l})
		}
	}
	rng.Shuffle(len(cs), func(i, j int) { cs[i], cs[j] = cs[j], cs[i] })
	return cs
}

func runRQ1() {
	log.Println("RQ1: seat over-allocation")
	trials := newCSV("rq1_trials.csv", "trial_id", "arm", "n", "capacity", "rep", "warmup", "order", "started_at", "wall_ms", "successes", "full", "errors", "enrolled_final", "overbooked")
	reqs := newCSV("rq1_requests.csv", "trial_id", "arm", "n", "rep", "warmup", "req", "offset_ms", "latency_ms", "outcome", "error")
	order := 0
	for rep := 0; rep < *warmup+*reps; rep++ {
		for _, c := range shuffledCells([]string{"naive", "locked"}, rq1Levels) {
			order++
			course := newCourse(capacity)
			fn := naiveEnroll
			if c.arm == "locked" {
				fn = lockedEnroll
			}
			type res struct {
				offset, lat time.Duration
				err         error
			}
			out := make([]res, c.level)
			gate := make(chan struct{})
			var wg sync.WaitGroup
			var t0 time.Time
			for i := 0; i < c.level; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-gate
					s := time.Now()
					err := fn(students[i], course)
					out[i] = res{s.Sub(t0), time.Since(s), err}
				}(i)
			}
			time.Sleep(30 * time.Millisecond) // let every goroutine park at the gate
			startedAt := time.Now().UTC()
			t0 = time.Now()
			close(gate)
			wg.Wait()
			wall := time.Since(t0)

			var final int64
			must(db.DB.Model(&models.StudentCourse{}).Where("course_id = ?", course.ID).Count(&final).Error)
			ok, full, bad := 0, 0, 0
			id := fmt.Sprintf("rq1-%04d", order)
			w := boolStr(rep < *warmup)
			for i, r := range out {
				o := classify(r.err)
				switch {
				case o == "ok":
					ok++
				case o == "full":
					full++
				default:
					bad++
				}
				msg := ""
				if o != "ok" && o != "full" {
					msg = r.err.Error()
				}
				reqs.row(id, c.arm, c.level, rep, w, i, ms(r.offset), ms(r.lat), o, msg)
			}
			over := int(final) - capacity
			if over < 0 {
				over = 0
			}
			trials.row(id, c.arm, c.level, capacity, rep, w, order, startedAt.Format(time.RFC3339Nano), ms(wall), ok, full, bad, final, over)
		}
		log.Printf("  rep %d/%d", rep+1, *warmup+*reps)
	}
	trials.close()
	reqs.close()
}

// ─────────────────────────────────────────────
//	RQ2: project lost updates
// ─────────────────────────────────────────────

var errTaken = errors.New("project_taken")

// legacyAssign is the original AssignProject: read, check, then Save the whole row.
func legacyAssign(st models.Student, p models.Project) error {
	var student models.Student
	if err := db.DB.Where("id = ?", st.ID).First(&student).Error; err != nil {
		return err
	}
	var project models.Project
	if err := db.DB.Where("id = ?", p.ID).First(&project).Error; err != nil {
		return err
	}
	if project.StudentID != nil {
		return errTaken
	}
	var existing models.Project
	err := db.DB.Where("student_id = ?", student.ID).First(&existing).Error
	if err == nil {
		return errTaken
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	project.StudentID = &student.ID
	return db.DB.Save(&project).Error
}

func atomicAssign(st models.Student, p models.Project) error {
	return svc.AssignProject(st.ID.String(), p.ID.String())
}

func runRQ2() {
	log.Println("RQ2: project lost updates")
	trials := newCSV("rq2_trials.csv", "trial_id", "arm", "k", "rep", "warmup", "order", "started_at", "wall_ms", "successes", "conflicts", "errors", "final_owner_set", "phantom_winners", "lost_updates")
	reqs := newCSV("rq2_requests.csv", "trial_id", "arm", "k", "rep", "warmup", "req", "latency_ms", "outcome", "error")
	order := 0
	for rep := 0; rep < *warmup+*reps; rep++ {
		for _, c := range shuffledCells([]string{"legacy", "atomic"}, rq2Levels) {
			order++
			p := newProject()
			fn := legacyAssign
			if c.arm == "atomic" {
				fn = atomicAssign
			}
			errs := make([]error, c.level)
			lats := make([]time.Duration, c.level)
			gate := make(chan struct{})
			var wg sync.WaitGroup
			for i := 0; i < c.level; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-gate
					s := time.Now()
					errs[i] = fn(students[i], p)
					lats[i] = time.Since(s)
				}(i)
			}
			time.Sleep(30 * time.Millisecond)
			startedAt := time.Now().UTC()
			t0 := time.Now()
			close(gate)
			wg.Wait()
			wall := time.Since(t0)

			var after models.Project
			must(db.DB.Where("id = ?", p.ID).First(&after).Error)
			id := fmt.Sprintf("rq2-%04d", order)
			w := boolStr(rep < *warmup)
			ok, conf, bad, phantom := 0, 0, 0, 0
			for i, err := range errs {
				var se *services.Error
				o, msg := "ok", ""
				switch {
				case err == nil:
					ok++
					if after.StudentID == nil || *after.StudentID != students[i].ID {
						phantom++ // told "assigned" but someone else owns it
					}
				case errors.Is(err, errTaken), errors.As(err, &se):
					conf++
					o = "conflict"
				default:
					bad++
					o, msg = "error", err.Error()
				}
				reqs.row(id, c.arm, c.level, rep, w, i, ms(lats[i]), o, msg)
			}
			lost := ok - 1
			if lost < 0 {
				lost = 0
			}
			trials.row(id, c.arm, c.level, rep, w, order, startedAt.Format(time.RFC3339Nano), ms(wall), ok, conf, bad, boolStr(after.StudentID != nil), phantom, lost)
			// free the claimants for the next trial
			must(db.DB.Model(&models.Project{}).Where("id = ?", p.ID).Update("student_id", nil).Error)
		}
		log.Printf("  rep %d/%d", rep+1, *warmup+*reps)
	}
	trials.close()
	reqs.close()
}

// ─────────────────────────────────────────────
//	RQ4: SSE push vs polling, through the running service
// ─────────────────────────────────────────────

type hit struct {
	client int
	at     time.Time
	val    int
}

func trigger(client *http.Client, st models.Student, c models.Course, enroll bool) (time.Duration, error) {
	var req *http.Request
	if enroll {
		req, _ = http.NewRequest("POST", fmt.Sprintf("%s/students/%s/courses", *apiBase, st.StudentID),
			strings.NewReader(fmt.Sprintf(`{"course_ids":["%s"]}`, c.CourseID)))
	} else {
		req, _ = http.NewRequest("DELETE", fmt.Sprintf("%s/students/%s/courses/%s", *apiBase, st.StudentID, c.CourseID), nil)
	}
	s := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}
	return time.Since(s), nil
}

func runRQ4() {
	log.Println("RQ4: SSE vs polling")
	c := newCourse(1000)
	st := students[0]
	client := &http.Client{Timeout: 10 * time.Second}
	out := newCSV("rq4_delays.csv", "mode", "level", "event", "client", "action", "sent_at", "post_ms", "delay_ms", "delivered")

	levels := append([]int(nil), sseLevels...)
	rng.Shuffle(len(levels), func(i, j int) { levels[i], levels[j] = levels[j], levels[i] })
	enrolled := false
	for _, s := range levels {
		ctx, cancel := context.WithCancel(context.Background())
		hits := make(chan hit, 4096)
		ready := make(chan struct{}, s)
		for i := 0; i < s; i++ {
			go sseClient(ctx, i, c.CourseID, hits, ready)
		}
		for i := 0; i < s; i++ {
			select {
			case <-ready:
			case <-time.After(10 * time.Second):
				log.Fatalf("SSE client %d never connected", i)
			}
		}
		for e := 0; e < sseEvents; e++ {
			enrolled = !enrolled
			sent := time.Now()
			post, err := trigger(client, st, c, enrolled)
			must(err)
			got := map[int]time.Duration{}
			deadline := time.After(3 * time.Second)
		collect:
			for len(got) < s {
				select {
				case h := <-hits:
					if _, dup := got[h.client]; !dup {
						got[h.client] = h.at.Sub(sent)
					}
				case <-deadline:
					break collect
				}
			}
			for i := 0; i < s; i++ {
				d, ok := got[i]
				out.row("sse", s, e, i, action(enrolled), sent.UTC().Format(time.RFC3339Nano), ms(post), ms(d), boolStr(ok))
			}
			time.Sleep(time.Duration(50+rng.Intn(100)) * time.Millisecond)
		}
		cancel()
		time.Sleep(300 * time.Millisecond)
		log.Printf("  sse S=%d done", s)
	}

	for _, T := range pollLevels {
		ctx, cancel := context.WithCancel(context.Background())
		hits := make(chan hit, 4096)
		initial := 0
		if enrolled {
			initial = 1
		}
		for i := 0; i < pollers; i++ {
			phase := time.Duration(rng.Int63n(int64(T) * int64(time.Millisecond)))
			go poller(ctx, i, c.CourseID, time.Duration(T)*time.Millisecond, phase, initial, hits)
		}
		time.Sleep(time.Duration(T) * time.Millisecond)
		for e := 0; e < pollEvents; e++ {
			enrolled = !enrolled
			want := 0
			if enrolled {
				want = 1
			}
			sent := time.Now()
			post, err := trigger(client, st, c, enrolled)
			must(err)
			got := map[int]time.Duration{}
			deadline := time.After(time.Duration(T)*time.Millisecond + 3*time.Second)
		pcollect:
			for len(got) < pollers {
				select {
				case h := <-hits:
					if _, dup := got[h.client]; !dup && h.val == want {
						got[h.client] = h.at.Sub(sent)
					}
				case <-deadline:
					break pcollect
				}
			}
			for i := 0; i < pollers; i++ {
				d, ok := got[i]
				out.row("poll", T, e, i, action(enrolled), sent.UTC().Format(time.RFC3339Nano), ms(post), ms(d), boolStr(ok))
			}
			time.Sleep(time.Duration(50+rng.Intn(100)) * time.Millisecond)
		}
		cancel()
		log.Printf("  poll T=%dms done", T)
	}
	out.close()
}

func action(enrolled bool) string {
	if enrolled {
		return "enroll"
	}
	return "drop"
}

func sseClient(ctx context.Context, id int, course string, hits chan<- hit, ready chan<- struct{}) {
	req, _ := http.NewRequestWithContext(ctx, "GET", *apiBase+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("sse client %d: %v", id, err)
		}
		return
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		now := time.Now()
		var e struct {
			Type string                 `json:"type"`
			Data map[string]interface{} `json:"data"`
		}
		if json.Unmarshal([]byte(line[6:]), &e) != nil {
			continue
		}
		if e.Type == "connected" {
			ready <- struct{}{}
			continue
		}
		if strings.HasPrefix(e.Type, "enrollment.") && e.Data["course_id"] == course {
			hits <- hit{client: id, at: now}
		}
	}
}

func poller(ctx context.Context, id int, course string, every, phase time.Duration, last int, hits chan<- hit) {
	client := &http.Client{Timeout: 5 * time.Second}
	// random phase so pollers are not synchronised with the trigger
	select {
	case <-ctx.Done():
		return
	case <-time.After(phase):
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		resp, err := client.Get(fmt.Sprintf("%s/courses/%s", *apiBase, course))
		if err == nil {
			var c struct {
				Enrolled int `json:"enrolled"`
			}
			json.NewDecoder(resp.Body).Decode(&c)
			resp.Body.Close()
			now := time.Now()
			if c.Enrolled != last {
				last = c.Enrolled
				hits <- hit{client: id, at: now, val: c.Enrolled}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// ─────────────────────────────────────────────
//	data management
// ─────────────────────────────────────────────

type csvFile struct {
	f *os.File
	w *csv.Writer
}

func newCSV(name string, header ...string) *csvFile {
	f, err := os.Create(filepath.Join(*outDir, name))
	must(err)
	w := csv.NewWriter(f)
	must(w.Write(header))
	return &csvFile{f, w}
}

func (c *csvFile) row(vals ...interface{}) {
	rec := make([]string, len(vals))
	for i, v := range vals {
		rec[i] = fmt.Sprint(v)
	}
	must(c.w.Write(rec))
}

func (c *csvFile) close() {
	c.w.Flush()
	must(c.w.Error())
	must(c.f.Close())
}

func writeManifest(start, end time.Time) {
	var pg string
	db.DB.Raw("SELECT version()").Scan(&pg)
	m := map[string]interface{}{
		"run_tag": tag, "started_at": start, "finished_at": end,
		"go_version": runtime.Version(), "goos": runtime.GOOS, "goarch": runtime.GOARCH,
		"num_cpu": runtime.NumCPU(), "gomaxprocs": runtime.GOMAXPROCS(0), "postgres": pg,
		"isolation": "READ COMMITTED (PostgreSQL default)",
		"params": map[string]interface{}{
			"seed": *seed, "reps": *reps, "warmup": *warmup, "pool": *poolSize, "capacity": capacity,
			"rq1_levels": rq1Levels, "rq2_levels": rq2Levels, "sse_levels": sseLevels, "poll_ms": pollLevels,
			"sse_events": sseEvents, "poll_events": pollEvents, "pollers": pollers, "api": *apiBase,
		},
		"source_sha256": sourceHash("."),
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	must(os.WriteFile(filepath.Join(*outDir, "manifest.json"), b, 0o644))
}

// sourceHash fingerprints every .go file so results can be tied to the exact code.
func sourceHash(root string) string {
	var files []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".go") {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)
	h := sha256.New()
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err == nil {
			h.Write([]byte(filepath.ToSlash(f)))
			h.Write(b)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func cleanupRows() {
	like := tag + "-%"
	db.DB.Exec(`DELETE FROM activities WHERE message LIKE ?`, "%"+tag+"-%")
	db.DB.Exec(`DELETE FROM courses WHERE course_id LIKE ?`, like)
	db.DB.Exec(`DELETE FROM projects WHERE project_id LIKE ?`, like)
	db.DB.Exec(`DELETE FROM students WHERE student_id LIKE ?`, like)
	log.Printf("cleaned up rows tagged %s", tag)
}

func ms(d time.Duration) string { return strconv.FormatFloat(float64(d)/1e6, 'f', 3, 64) }
func boolStr(b bool) string     { return strconv.FormatBool(b) }

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
