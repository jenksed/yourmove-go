package app

import (
	"errors"
	"log"
	"net/http"
)

// importJobs is a bounded local-only operation. Metadata enrichment remains an
// explicit per-job action, avoiding dozens of surprise external requests.
func (a *App) importJobs(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	if err := r.ParseForm(); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Import payload too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "Invalid batch form", http.StatusBadRequest)
		}
		return
	}
	if len(r.PostForm.Get("job_urls")) > 125<<10 {
		http.Error(w, "Too much input", http.StatusRequestEntityTooLarge)
		return
	}
	results, err := a.importJobBatch(r.PostForm.Get("job_urls"))
	if err != nil {
		if errors.Is(err, errBatchInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		log.Printf("batch intake: %v", err)
		http.Error(w, "Unable to import jobs; no changes from this batch were saved", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := a.resources.batchResultsTemplate.Execute(w, results); err != nil {
		log.Printf("render batch intake: %v", err)
	}
}
