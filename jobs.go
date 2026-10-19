package main

import "fmt"

type Job struct {
	Title string
	URL   string
	Host  string
}

func (a *App) saveJob(job Job) error {
	_, err := a.db.Exec(
		"INSERT INTO jobs (url, host, title) VALUES (?, ?, ?)",
		job.URL,
		job.Host,
		job.Title,
	)

	if err != nil {
		return fmt.Errorf("save job: %w", err)
	}

	return nil
}

func (a *App) allJobs() ([]Job, error) {
	rows, err := a.db.Query(
		"SELECT url, host, title FROM jobs ORDER BY id",
	)
	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job

		err := rows.Scan(&job.URL, &job.Host, &job.Title)
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
