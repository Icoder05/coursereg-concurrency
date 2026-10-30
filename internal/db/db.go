package db

import (
	"log"

	"go-student-service/internal/config"
	"go-student-service/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// By passing a pointer (*http.Request), only the memory address of the request struct is copied
func Connect(cfg *config.Config) error {
	dsn := cfg.PostgresDSN() //PostgresDSN refers to the Data Source Name (DSN) required to establish a connection between an application (like a Go program) and a PostgreSQL database
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return err
	}

	// Order matters: referenced tables first so FKs can be created.
	if err := DB.AutoMigrate(
		&models.Student{}, &models.Course{}, &models.Project{},
		&models.StudentCourse{}, &models.WaitlistEntry{}, &models.Activity{},
	); err != nil {
		return err
	}

	// projects.student_id -> students.id; unassign the project if the student is deleted
	if err := DB.Exec(`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_projects_student') THEN
			ALTER TABLE projects ADD CONSTRAINT fk_projects_student
				FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE SET NULL;
		END IF;
	END $$`).Error; err != nil {
		return err
	}

	// create partial/unique indexes
	if err := DB.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_student_unique ON projects(student_id) WHERE student_id IS NOT NULL`).Error; err != nil {
		return err
	}
	if err := DB.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_student_courses_student_course_unique ON student_courses(student_id, course_id)`).Error; err != nil {
		return err
	}

	log.Println("migrations completed")
	return nil
}
