package store

import (
	"fmt"
	"time"
)

// CronJob is a scheduled command managed by the panel.
type CronJob struct {
	ID        int64
	Schedule  string // standard 5-field cron expression
	Command   string
	Comment   string
	CreatedAt time.Time
}

// CreateCronJob inserts a cron job.
func (s *Store) CreateCronJob(schedule, command, comment string) (CronJob, error) {
	j := CronJob{Schedule: schedule, Command: command, Comment: comment, CreatedAt: now()}
	res, err := s.db.Exec(
		`INSERT INTO cron_jobs (schedule, command, comment, created_at) VALUES (?, ?, ?, ?)`,
		j.Schedule, j.Command, j.Comment, j.CreatedAt)
	if err != nil {
		return CronJob{}, fmt.Errorf("create cron job: %w", err)
	}
	j.ID, _ = res.LastInsertId()
	return j, nil
}

// CronJobs lists all cron jobs.
func (s *Store) CronJobs() ([]CronJob, error) {
	rows, err := s.db.Query(`SELECT id, schedule, command, comment, created_at FROM cron_jobs ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list cron jobs: %w", err)
	}
	defer rows.Close()
	var out []CronJob
	for rows.Next() {
		var j CronJob
		if err := rows.Scan(&j.ID, &j.Schedule, &j.Command, &j.Comment, &j.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// DeleteCronJob removes a cron job.
func (s *Store) DeleteCronJob(id int64) error {
	return s.mustAffect(s.db.Exec(`DELETE FROM cron_jobs WHERE id = ?`, id))
}
