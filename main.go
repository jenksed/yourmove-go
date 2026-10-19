package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
)

type App struct {
	db *sql.DB
}

func main() {
	db, err := openDB("jobs.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	app := &App{db: db}

	fmt.Println("Open http://127.0.0.1:3000")
	if err := http.ListenAndServe("127.0.0.1:3000", newMux(app)); err != nil {
		log.Fatal(err)
	}
}

func newMux(app *App) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("/", home)
	mux.HandleFunc("/jobs", app.jobForm)
	mux.HandleFunc("GET /jobs/{id}", app.jobDetail)
	return mux
}
