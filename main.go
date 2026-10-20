//go:build !desktop

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	dbPath, resumeDir, err := applicationStorage()
	if err != nil {
		log.Fatal(err)
	}
	db, err := openDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	app := &App{db: db, resumeDir: resumeDir}
	defer app.browsers.closeAll()

	fmt.Println("Open http://127.0.0.1:3000")
	server := &http.Server{Addr: "127.0.0.1:3000", Handler: newMux(app)}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	select {
	case err := <-done:
		if err != nil && err != http.ErrServerClosed {
			log.Printf("YourMove HTTP server: %v", err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}
}
