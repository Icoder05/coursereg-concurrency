Auto migrations are performed by GORM at startup (internal/db/db.go). Additional SQL runs there too:
- FK projects.student_id -> students.id (ON DELETE SET NULL)
- partial unique index on projects.student_id WHERE student_id IS NOT NULL
- unique index on student_courses(student_id, course_id)

Schema changed (capacity, credits, waitlist, activities). If you have an old database volume, reset it with `docker compose down -v`.
