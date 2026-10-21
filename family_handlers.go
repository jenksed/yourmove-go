package main

import (
	"database/sql"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
)

var familiesTemplate = template.Must(template.ParseFS(templateFiles, "templates/families.html", "templates/partials.html"))
var utilitiesTemplate = template.Must(template.ParseFS(templateFiles, "templates/utilities.html", "templates/partials.html"))

type familyPageData struct {
	Families           []RoleFamily
	Preview            *MatchRecommendation
	PreviewTitle       string
	PreviewDescription string
}

func (a *App) renderFamilies(w http.ResponseWriter, preview *familyPageData) {
	families, err := a.allFamilies()
	if err != nil {
		log.Printf("families: %v", err)
		http.Error(w, "Cannot load families", 500)
		return
	}
	data := familyPageData{Families: families}
	if preview != nil {
		data.Preview = preview.Preview
		data.PreviewTitle = preview.PreviewTitle
		data.PreviewDescription = preview.PreviewDescription
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := familiesTemplate.Execute(w, data); err != nil {
		log.Printf("render families: %v", err)
	}
}

func (a *App) familyPage(w http.ResponseWriter, r *http.Request) { a.renderFamilies(w, nil) }
func (a *App) utilitiesPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := utilitiesTemplate.Execute(w, nil); err != nil {
		log.Printf("render utilities: %v", err)
	}
}

func familyFromForm(r *http.Request) RoleFamily {
	return RoleFamily{Name: r.PostForm.Get("name"), TitleTerms: r.PostForm.Get("title_terms"), ContextTerms: r.PostForm.Get("context_terms"), ExcludeTerms: r.PostForm.Get("exclude_terms")}
}

func parseLocalForm(w http.ResponseWriter, r *http.Request, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid or oversized form", http.StatusBadRequest)
		return false
	}
	return true
}

func (a *App) createFamily(w http.ResponseWriter, r *http.Request) {
	if !parseLocalForm(w, r, 12<<10) {
		return
	}
	if err := a.saveFamily(familyFromForm(r)); err != nil {
		http.Error(w, "Cannot save role family: "+err.Error(), 400)
		return
	}
	http.Redirect(w, r, "/families", http.StatusSeeOther)
}

func (a *App) editFamily(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 12<<10) {
		return
	}
	f := familyFromForm(r)
	f.ID = id
	err = a.saveFamily(f)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Cannot update family: "+err.Error(), 400)
		return
	}
	http.Redirect(w, r, "/families", http.StatusSeeOther)
}

func (a *App) removeFamily(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 2<<10) {
		return
	}
	if r.PostForm.Get("confirm") != "yes" {
		http.Error(w, "Confirm family removal", 400)
		return
	}
	err = a.deleteFamily(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("remove family: %v", err)
		http.Error(w, "Cannot remove family", 500)
		return
	}
	http.Redirect(w, r, "/families", http.StatusSeeOther)
}

func (a *App) addExamples(w http.ResponseWriter, r *http.Request) {
	if !parseLocalForm(w, r, 2<<10) {
		return
	}
	if r.PostForm.Get("confirm") != "yes" {
		http.Error(w, "Confirm example installation", 400)
		return
	}
	if _, err := a.installExampleFamilies(); err != nil {
		log.Printf("examples: %v", err)
		http.Error(w, "Cannot add example families", 500)
		return
	}
	http.Redirect(w, r, "/families", http.StatusSeeOther)
}

func (a *App) testFamilyMatch(w http.ResponseWriter, r *http.Request) {
	if !parseLocalForm(w, r, 8<<10) {
		return
	}
	title := strings.TrimSpace(r.PostForm.Get("title"))
	description := strings.TrimSpace(r.PostForm.Get("description"))
	if len(title) > 200 || len(description) > 4000 {
		http.Error(w, "Preview text too long", 400)
		return
	}
	families, err := a.allFamilies()
	if err != nil {
		http.Error(w, "Cannot load rules", 500)
		return
	}
	resumes, err := a.allResumes()
	if err != nil {
		http.Error(w, "Cannot load documents", 500)
		return
	}
	match := matchResume(Job{Title: title, Description: description}, families, resumes)
	a.renderFamilies(w, &familyPageData{Preview: &match, PreviewTitle: title, PreviewDescription: description})
}

func (a *App) setResumeFamily(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	raw := r.PostForm.Get("family_id")
	familyID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || familyID < 0 {
		http.Error(w, "Invalid family", 400)
		return
	}
	err = a.assignResumeFamily(id, familyID, r.PostForm.Get("primary") == "yes")
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Unknown resume or family", 404)
		return
	}
	if err != nil {
		log.Printf("assign resume family: %v", err)
		http.Error(w, "Cannot update resume routing", 500)
		return
	}
	http.Redirect(w, r, "/resumes", http.StatusSeeOther)
}
