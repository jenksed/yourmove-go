package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	_ "modernc.org/sqlite"
)

type App struct {
	db *sql.DB
}

func main() {
	db, err := sql.Open("sqlite", "jobs.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS jobs (
			id INTEGER PRIMARY KEY,
			url TEXT NOT NULL,
			host TEXT NOT NULL
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	app := &App{db: db}

	http.HandleFunc("/", home)
	http.HandleFunc("/jobs", app.jobForm)

	fmt.Println("Open http://127.0.0.1:3000")

	if err := http.ListenAndServe("127.0.0.1:3000", nil); err != nil {
		log.Fatal(err)
	}
}
