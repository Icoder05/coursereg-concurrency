// Package seed fills an empty database with a realistic, randomised college:
// courses with seat limits, students with credit ceilings, capstone projects,
// and an initial spread of enrollments that respects every rule.
package seed

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"go-student-service/internal/db"
	"go-student-service/internal/models"
)

var courses = []models.Course{
	{CourseID: "CS101", Name: "Introduction to Programming", Department: "CSE", Credits: 4, Capacity: 60, Instructor: "Dr. Meera Iyer", Schedule: "Mon/Wed 09:00"},
	{CourseID: "CS201", Name: "Data Structures", Department: "CSE", Credits: 4, Capacity: 50, Instructor: "Prof. Arjun Rao", Schedule: "Tue/Thu 10:30"},
	{CourseID: "CS301", Name: "Operating Systems", Department: "CSE", Credits: 4, Capacity: 40, Instructor: "Dr. Kavya Menon", Schedule: "Mon/Wed 11:00"},
	{CourseID: "CS305", Name: "Database Systems", Department: "CSE", Credits: 3, Capacity: 40, Instructor: "Dr. Rohan Gupta", Schedule: "Tue/Thu 14:00"},
	{CourseID: "CS340", Name: "Computer Networks", Department: "CSE", Credits: 3, Capacity: 35, Instructor: "Prof. Sanjay Pillai", Schedule: "Fri 09:00"},
	{CourseID: "CS410", Name: "Machine Learning", Department: "CSE", Credits: 4, Capacity: 30, Instructor: "Dr. Ananya Bose", Schedule: "Mon/Wed 15:00"},
	{CourseID: "CS450", Name: "Cloud Computing", Department: "CSE", Credits: 3, Capacity: 25, Instructor: "Dr. Vikram Shah", Schedule: "Thu 16:00"},
	{CourseID: "CS499", Name: "Quantum Computing Seminar", Department: "CSE", Credits: 2, Capacity: 12, Instructor: "Prof. Leela Nair", Schedule: "Fri 15:00"},
	{CourseID: "EC210", Name: "Digital Logic Design", Department: "ECE", Credits: 4, Capacity: 45, Instructor: "Dr. Harish Kumar", Schedule: "Tue/Thu 09:00"},
	{CourseID: "EC320", Name: "Signals and Systems", Department: "ECE", Credits: 3, Capacity: 35, Instructor: "Prof. Divya Reddy", Schedule: "Mon/Wed 13:00"},
	{CourseID: "EC415", Name: "VLSI Design", Department: "ECE", Credits: 3, Capacity: 20, Instructor: "Dr. Nikhil Joshi", Schedule: "Fri 11:00"},
	{CourseID: "MA101", Name: "Calculus I", Department: "MATH", Credits: 4, Capacity: 70, Instructor: "Prof. Suresh Patel", Schedule: "Mon/Wed/Fri 08:00"},
	{CourseID: "MA202", Name: "Linear Algebra", Department: "MATH", Credits: 3, Capacity: 50, Instructor: "Dr. Priya Das", Schedule: "Tue/Thu 11:30"},
	{CourseID: "MA305", Name: "Probability and Statistics", Department: "MATH", Credits: 3, Capacity: 45, Instructor: "Dr. Farhan Ali", Schedule: "Mon/Wed 14:00"},
	{CourseID: "ME150", Name: "Engineering Mechanics", Department: "ME", Credits: 3, Capacity: 40, Instructor: "Prof. Ravi Verma", Schedule: "Tue/Thu 08:00"},
	{CourseID: "ME330", Name: "Thermodynamics", Department: "ME", Credits: 4, Capacity: 30, Instructor: "Dr. Sneha Kulkarni", Schedule: "Mon/Wed 10:00"},
	{CourseID: "PH101", Name: "Physics I", Department: "PHY", Credits: 4, Capacity: 60, Instructor: "Dr. Imran Qureshi", Schedule: "Tue/Thu 13:00"},
	{CourseID: "HS201", Name: "Technical Writing", Department: "HSS", Credits: 2, Capacity: 30, Instructor: "Ms. Ritu Malhotra", Schedule: "Fri 13:00"},
	{CourseID: "HS310", Name: "Ethics in Technology", Department: "HSS", Credits: 2, Capacity: 25, Instructor: "Dr. Thomas George", Schedule: "Wed 16:00"},
}

var projects = []models.Project{
	{ProjectID: "P001", Name: "Campus Navigation AR App", Supervisor: "Dr. Ananya Bose"},
	{ProjectID: "P002", Name: "Crop Disease Detection with CNNs", Supervisor: "Dr. Ananya Bose"},
	{ProjectID: "P003", Name: "Distributed Key-Value Store", Supervisor: "Dr. Kavya Menon"},
	{ProjectID: "P004", Name: "Smart Energy Meter on FPGA", Supervisor: "Dr. Nikhil Joshi"},
	{ProjectID: "P005", Name: "Real-time Bus Tracking System", Supervisor: "Prof. Sanjay Pillai"},
	{ProjectID: "P006", Name: "Library Recommendation Engine", Supervisor: "Dr. Rohan Gupta"},
	{ProjectID: "P007", Name: "Drone Swarm Path Planning", Supervisor: "Prof. Ravi Verma"},
	{ProjectID: "P008", Name: "Sign Language Translator", Supervisor: "Dr. Vikram Shah"},
	{ProjectID: "P009", Name: "Blockchain Certificate Verification", Supervisor: "Prof. Arjun Rao"},
	{ProjectID: "P010", Name: "Air Quality IoT Network", Supervisor: "Dr. Harish Kumar"},
	{ProjectID: "P011", Name: "Exam Timetable Optimiser", Supervisor: "Dr. Farhan Ali"},
	{ProjectID: "P012", Name: "Solar Tracker Control System", Supervisor: "Dr. Sneha Kulkarni"},
	{ProjectID: "P013", Name: "Mental Health Chat Companion", Supervisor: "Dr. Thomas George"},
	{ProjectID: "P014", Name: "Compiler for a Teaching Language", Supervisor: "Dr. Meera Iyer"},
	{ProjectID: "P015", Name: "Quantum Circuit Simulator", Supervisor: "Prof. Leela Nair"},
	{ProjectID: "P016", Name: "Hostel Mess Feedback Analytics", Supervisor: "Dr. Priya Das"},
	{ProjectID: "P017", Name: "Wind Turbine Blade Stress Analysis", Supervisor: "Prof. Ravi Verma"},
	{ProjectID: "P018", Name: "Network Intrusion Detection", Supervisor: "Prof. Sanjay Pillai"},
}

var firstNames = []string{"Aarav", "Vivaan", "Aditya", "Ishaan", "Kabir", "Arjun", "Reyansh", "Sai", "Rohan", "Dhruv",
	"Ananya", "Diya", "Saanvi", "Aadhya", "Kiara", "Myra", "Ira", "Sara", "Meera", "Riya",
	"Krishna", "Harsha", "Nikhil", "Pranav", "Tanvi", "Sneha", "Pooja", "Lakshmi", "Neha", "Zoya",
	"Omar", "Daniel", "Emma", "Liam", "Noah", "Olivia", "Ayesha", "Farhan", "Yash", "Aditi"}

var lastNames = []string{"Sharma", "Reddy", "Iyer", "Nair", "Patel", "Gupta", "Rao", "Menon", "Das", "Khan",
	"Singh", "Joshi", "Kulkarni", "Chatterjee", "Pillai", "Varma", "Bose", "Mehta", "Thomas", "Fernandes"}

var departments = []string{"CSE", "CSE", "CSE", "ECE", "ECE", "ME", "MATH", "PHY"}

// Run seeds demo data if the database has no courses yet.
func Run() error {
	var n int64
	if err := db.DB.Model(&models.Course{}).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	cs := make([]models.Course, len(courses))
	copy(cs, courses)
	if err := db.DB.Create(&cs).Error; err != nil {
		return err
	}

	students := make([]models.Student, 0, 90)
	used := map[string]bool{}
	seq := map[string]int{}
	for len(students) < 90 {
		first, last := firstNames[rng.Intn(len(firstNames))], lastNames[rng.Intn(len(lastNames))]
		name := first + " " + last
		if used[name] {
			continue
		}
		used[name] = true
		dept := departments[rng.Intn(len(departments))]
		year := 1 + rng.Intn(4)
		seq[dept]++
		students = append(students, models.Student{
			StudentID:  fmt.Sprintf("%d%s%03d", 2026-year, dept, seq[dept]),
			Name:       name,
			Email:      strings.ToLower(first+"."+last) + "@college.edu",
			Department: dept,
			Year:       year,
			MaxCredits: []int{18, 20, 22, 24}[rng.Intn(4)],
		})
	}
	if err := db.DB.Create(&students).Error; err != nil {
		return err
	}

	ps := make([]models.Project, len(projects))
	copy(ps, projects)
	if err := db.DB.Create(&ps).Error; err != nil {
		return err
	}

	// Random initial enrollments, tracked in memory so seats and credits stay valid.
	// CS499 is left empty so the "courses with no students" panel has something to show.
	seats := map[int]int{}
	var rows []models.StudentCourse
	for _, st := range students {
		credits, want := 0, 2+rng.Intn(4)
		for _, ci := range rng.Perm(len(cs)) {
			if want == 0 {
				break
			}
			c := cs[ci]
			if c.CourseID == "CS499" || seats[ci] >= c.Capacity || credits+c.Credits > st.MaxCredits {
				continue
			}
			// bias toward courses in the student's own department
			if c.Department != st.Department && rng.Intn(3) > 0 {
				continue
			}
			seats[ci]++
			credits += c.Credits
			want--
			rows = append(rows, models.StudentCourse{StudentID: st.ID, CourseID: c.ID,
				CreatedAt: time.Now().Add(-time.Duration(rng.Intn(72*60)) * time.Minute)})
		}
	}
	if err := db.DB.CreateInBatches(&rows, 200).Error; err != nil {
		return err
	}

	// Hand roughly half the projects to final-year students.
	var seniors []models.Student
	for _, st := range students {
		if st.Year >= 3 {
			seniors = append(seniors, st)
		}
	}
	rng.Shuffle(len(seniors), func(i, j int) { seniors[i], seniors[j] = seniors[j], seniors[i] })
	order := rng.Perm(len(ps))
	for i := 0; i < len(ps)/2 && i < len(seniors); i++ {
		if err := db.DB.Model(&ps[order[i]]).Update("student_id", seniors[i].ID).Error; err != nil {
			return err
		}
	}

	msg := fmt.Sprintf("Demo college seeded: %d courses, %d students, %d enrollments, %d projects",
		len(cs), len(students), len(rows), len(ps))
	db.DB.Create(&models.Activity{Type: "system", Message: msg})
	log.Println(msg)
	return nil
}
