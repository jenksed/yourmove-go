package main

import "fmt"

type Job struct {
	ID          int64
	Title       string
	URL         string
	Host        string
	Description string
}

func (a *App) saveJob(job Job) error {
	_, err := a.db.Exec(
		"INSERT INTO jobs (url, host, title, description) VALUES (?, ?, ?, ?)",
		job.URL,
		job.Host,
		job.Title,
		job.Description,
	)

	if err != nil {
		return fmt.Errorf("save job: %w", err)
	}

	return nil
}

func (a *App) allJobs() ([]Job, error) {
	rows, err := a.db.Query(
		"SELECT id, url, host, title, description FROM jobs ORDER BY id",
	)
	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job

		err := rows.Scan(
			&job.ID,
			&job.URL,
			&job.Host,
			&job.Title,
			&job.Description,
		)
		if err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}

		jobs = append(jobs, job)

	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs: %w", err)
	}
	return jobs, nil
}

func (a *App) jobByID(id int64) (Job, error) {
	var job Job

	err := a.db.QueryRow(
		"SELECT id, url, host, title, description FROM jobs WHERE id = ?",
		id,
	).Scan(
		&job.ID,
		&job.URL,
		&job.Host,
		&job.Title,
		&job.Description,
	)

	if err != nil {
		return Job{}, fmt.Errorf("load job %d: %w", id, err)
	}

	return job, nil
}
