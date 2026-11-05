//go:build !desktop

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jenksed/yourmove-go/internal/app"
	"github.com/jenksed/yourmove-go/internal/storage"
)

func main() {
	dbPath, resumeDir, err := storage.ApplicationStorage()
	if err != nil {
		log.Fatal(err)
	}
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	application, err := app.New(app.Config{
		DB: db, ResumeDir: resumeDir, TemplateFiles: templateFiles,
		StaticFiles: staticFiles, FrontendDist: frontendDist, CoverLetterInstructions: coverLetterInstructions, ApplicationAnswerInstructions: applicationAnswerInstructions,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer application.Close()

	port := os.Getenv("YOURMOVE_HTTP_PORT")
	if port == "" {
		port = "3000"
	}
	addr := net.JoinHostPort("127.0.0.1", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("YourMove HTTP server listen on %s: %v", addr, err)
	}
	fmt.Printf("Open http://%s\n", listener.Addr().String())
	server := &http.Server{Handler: application.Handler()}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
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
