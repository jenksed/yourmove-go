package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// answerLibraryListItem is the compact master-list row. The full answer stays
// in the detail pane so hundreds of entries remain browsable.
type answerLibraryListItem struct {
	ID        int64
	Question  string
	Employer  string
	JobTitle  string
	Host      string
	AltCount  int
	UpdatedAt string
	Selected  bool
	Link      string
}

type answerLibraryPage struct {
	Entries         []AnswerLibraryEntry
	SearchResults   []answerLibrarySuggestion
	SearchQuery     string
	SearchPerformed bool
	Notice          string
	// Master-detail browse state. List holds the compact current page;
	// Selected holds the full detail entry without discarding search/sort.
	List         []answerLibraryListItem
	AltCounts    map[int64]int
	Sort         string
	Page         int
	PerPage      int
	Total        int64
	TotalPages   int
	HasPrev      bool
	HasNext      bool
	PrevLink     string
	NextLink     string
	Selected     *AnswerLibraryEntry
	SelectedLink string
	BaseQuery    string
}

func answerLibrarySortParam(raw string) string {
	if strings.TrimSpace(raw) == "alpha" {
		return "alpha"
	}
	return "recent"
}

func answerLibraryPageParam(raw string) int {
	page, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || page < 1 {
		return 1
	}
	if page > 10000 {
		return 10000
	}
	return page
}

func answerLibraryDetailLink(query, sort string, page, selected int64) string {
	values := url.Values{}
	if strings.TrimSpace(query) != "" {
		values.Set("q", strings.TrimSpace(query))
	}
	if sort == "alpha" {
		values.Set("sort", "alpha")
	}
	if page > 1 {
		values.Set("page", strconv.FormatInt(page, 10))
	}
	if selected > 0 {
		values.Set("selected", strconv.FormatInt(selected, 10))
	}
	encoded := values.Encode()
	if encoded == "" {
		return "/answer-library"
	}
	return "/answer-library?" + encoded
}

func (a *App) renderAnswerLibraryList(w http.ResponseWriter, r *http.Request, query, sort string, page int, selectedID int64) {
	const perPage = answerLibraryLibraryPageSize
	notice := map[string]string{
		"saved":    "Saved as a separate Answer Library example. The employer-specific answer remains unchanged.",
		"existing": "This saved answer revision is already in the Answer Library. Its existing library copy was left unchanged.",
		"edited":   "The Answer Library copy was updated. The original application answer was left unchanged.",
		"deleted":  "The Answer Library example was deleted. Application history was left unchanged.",
	}[r.URL.Query().Get("notice")]
	data := answerLibraryPage{
		SearchQuery: query, SearchPerformed: strings.TrimSpace(query) != "",
		Notice: notice, Sort: sort, Page: page, PerPage: perPage,
		AltCounts: map[int64]int{},
	}
	if data.Sort == "" {
		data.Sort = "recent"
	}
	if data.Page < 1 {
		data.Page = 1
	}
	if data.SearchPerformed {
		if len(data.SearchQuery) > 1000 || !utf8.ValidString(data.SearchQuery) || strings.ContainsRune(data.SearchQuery, 0) {
			http.Error(w, "Search text is invalid or too long", http.StatusBadRequest)
			return
		}
		results, err := a.searchAnswerLibraryPaged(data.SearchQuery, answerLibraryLibrarySearchLimit)
		if err != nil {
			http.Error(w, "Unable to search the Answer Library", http.StatusInternalServerError)
			return
		}
		total := len(results)
		data.Total = int64(total)
		data.TotalPages = (total + perPage - 1) / perPage
		if data.TotalPages < 1 {
			data.TotalPages = 1
		}
		if data.Page > data.TotalPages {
			data.Page = data.TotalPages
		}
		start := (data.Page - 1) * perPage
		if start > total {
			start = total
		}
		end := start + perPage
		if end > total {
			end = total
		}
		pageResults := results[start:end]
		data.SearchResults = pageResults
		data.Total = int64(total)
		ids := make([]int64, 0, len(pageResults)+1)
		for _, item := range pageResults {
			ids = append(ids, item.EntryID)
		}
		if selectedID > 0 {
			ids = append(ids, selectedID)
		}
		if counts, err := a.answerLibraryAltCounts(ids); err == nil {
			data.AltCounts = counts
		}
		for _, item := range pageResults {
			employer := item.SourceEmployer
			if employer == "" {
				employer = item.SourceHost
			}
			data.List = append(data.List, answerLibraryListItem{
				ID: item.EntryID, Question: item.OriginalQuestion,
				Employer: employer, JobTitle: item.SourceJobTitle, Host: item.SourceHost,
				AltCount: data.AltCounts[item.EntryID], Selected: item.EntryID == selectedID,
				Link: answerLibraryDetailLink(data.SearchQuery, data.Sort, int64(data.Page), item.EntryID),
			})
		}
	} else {
		entries, total, err := a.answerLibraryListPage(data.Sort, data.Page, perPage)
		if err != nil {
			http.Error(w, "Unable to load the Answer Library", http.StatusInternalServerError)
			return
		}
		data.Total = total
		data.TotalPages = int((total + int64(perPage) - 1) / int64(perPage))
		if data.TotalPages < 1 {
			data.TotalPages = 1
		}
		if data.Page > data.TotalPages {
			data.Page = data.TotalPages
			entries, total, err = a.answerLibraryListPage(data.Sort, data.Page, perPage)
			if err != nil {
				http.Error(w, "Unable to load the Answer Library", http.StatusInternalServerError)
				return
			}
			data.Total = total
		}
		data.Entries = entries
		ids := make([]int64, 0, len(entries)+1)
		for _, entry := range entries {
			ids = append(ids, entry.ID)
		}
		if selectedID > 0 {
			ids = append(ids, selectedID)
		}
		if counts, err := a.answerLibraryAltCounts(ids); err == nil {
			data.AltCounts = counts
		}
		for _, entry := range entries {
			employer := entry.SourceEmployer
			if employer == "" {
				employer = entry.SourceHost
			}
			data.List = append(data.List, answerLibraryListItem{
				ID: entry.ID, Question: entry.OriginalQuestion,
				Employer: employer, JobTitle: entry.SourceJobTitle, Host: entry.SourceHost,
				AltCount: data.AltCounts[entry.ID], UpdatedAt: entry.UpdatedAt,
				Selected: entry.ID == selectedID,
				Link:     answerLibraryDetailLink("", data.Sort, int64(data.Page), entry.ID),
			})
		}
	}
	if selectedID > 0 {
		entry, err := a.answerLibraryBySourceID(a.db, selectedID)
		if err == nil {
			alternatives, err := a.libraryAlternatives(entry.ID)
			if err == nil {
				entry.Alternatives = alternatives
				data.Selected = &entry
				data.SelectedLink = answerLibraryDetailLink(data.SearchQuery, data.Sort, int64(data.Page), 0)
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Unable to load this Answer Library example", http.StatusInternalServerError)
			return
		}
	}
	// Default detail keeps the full answer visible without a selection round
	// trip and preserves the existing single-entry rendering expectation.
	if data.Selected == nil && len(data.List) > 0 {
		first := data.List[0].ID
		if entry, err := a.answerLibraryBySourceID(a.db, first); err == nil {
			if alternatives, err := a.libraryAlternatives(entry.ID); err == nil {
				entry.Alternatives = alternatives
				data.Selected = &entry
				data.SelectedLink = answerLibraryDetailLink(data.SearchQuery, data.Sort, int64(data.Page), 0)
				data.List[0].Selected = true
				selectedID = first
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Unable to load this Answer Library example", http.StatusInternalServerError)
			return
		}
	}
	data.HasPrev = data.Page > 1
	data.HasNext = data.Page < data.TotalPages
	if data.HasPrev {
		data.PrevLink = answerLibraryDetailLink(data.SearchQuery, data.Sort, int64(data.Page-1), selectedID)
	}
	if data.HasNext {
		data.NextLink = answerLibraryDetailLink(data.SearchQuery, data.Sort, int64(data.Page+1), selectedID)
	}
	values := url.Values{}
	if data.SearchPerformed {
		values.Set("q", data.SearchQuery)
	}
	if data.Sort == "alpha" {
		values.Set("sort", "alpha")
	}
	if data.Page > 1 {
		values.Set("page", strconv.Itoa(data.Page))
	}
	data.BaseQuery = values.Encode()
	a.renderAnswerLibrary(w, data)
}

func (a *App) answerLibrary(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	a.renderAnswerLibraryList(w, r, strings.TrimSpace(query.Get("q")), answerLibrarySortParam(query.Get("sort")), answerLibraryPageParam(query.Get("page")), func() int64 {
		selected, _ := strconv.ParseInt(strings.TrimSpace(query.Get("selected")), 10, 64)
		if selected < 0 {
			return 0
		}
		return selected
	}())
}

func (a *App) renderAnswerLibrary(w http.ResponseWriter, data answerLibraryPage) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	if err := a.resources.answerLibraryTemplate.Execute(w, data); err != nil {
		http.Error(w, "Unable to render the Answer Library", http.StatusInternalServerError)
	}
}

func (a *App) searchAnswerLibraryHandler(w http.ResponseWriter, r *http.Request) {
	if !parseLocalForm(w, r, 2<<10) {
		return
	}
	values := r.PostForm["query"]
	if len(values) != 1 || len(values[0]) > 1000 || !utf8.ValidString(values[0]) || strings.ContainsRune(values[0], 0) {
		http.Error(w, "Search text is invalid or too long", http.StatusBadRequest)
		return
	}
	query := strings.TrimSpace(values[0])
	if query == "" {
		http.Redirect(w, r, "/answer-library", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/answer-library?q="+url.QueryEscape(query), http.StatusSeeOther)
}

func (a *App) answerLibraryEntryHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	if _, err := a.answerLibraryBySourceID(a.db, id); errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, "Unable to load this Answer Library example", http.StatusInternalServerError)
		return
	}
	query := r.URL.Query()
	a.renderAnswerLibraryList(w, r, strings.TrimSpace(query.Get("q")), answerLibrarySortParam(query.Get("sort")), answerLibraryPageParam(query.Get("page")), id)
}

func (a *App) answerLibraryEntryContentHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	entry, err := a.answerLibraryBySourceID(a.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Unable to load this Answer Library example", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	kind := entry.SourceKind
	if kind == "" {
		kind = "live"
	}
	_ = json.NewEncoder(w).Encode(struct {
		Revision              int64  `json:"revision"`
		OriginalQuestion      string `json:"original_question"`
		Answer                string `json:"answer"`
		SourceEmployer        string `json:"source_employer"`
		SourceJobTitle        string `json:"source_job_title"`
		SourceHost            string `json:"source_host"`
		SourceKind            string `json:"source_kind"`
		EarlyDiscoverySource  string `json:"early_discovery_source"`
		EarlySourceQuestionID string `json:"early_source_question_id"`
		CreatedAt             string `json:"created_at"`
		UpdatedAt             string `json:"updated_at"`
	}{entry.Revision, entry.OriginalQuestion, entry.Answer, entry.SourceEmployer, entry.SourceJobTitle, entry.SourceHost, kind, entry.EarlyDiscoverySource, entry.EarlySourceQuestionID, entry.CreatedAt, entry.UpdatedAt})
}

func (a *App) answerLibraryStartingPointHandler(w http.ResponseWriter, r *http.Request) {
	jobID, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	entryValues, keyValues, revisionValues, libraryRevisionValues := r.PostForm["library_entry_id"], r.PostForm["question_key"], r.PostForm["revision"], r.PostForm["library_revision"]
	if len(entryValues) != 1 || len(keyValues) != 1 || len(revisionValues) != 1 || len(libraryRevisionValues) != 1 || !questionReturnKey.MatchString(keyValues[0]) {
		http.Error(w, "Review the current question and selected example", http.StatusBadRequest)
		return
	}
	entryID, entryErr := strconv.ParseInt(entryValues[0], 10, 64)
	revision, revisionErr := strconv.ParseInt(revisionValues[0], 10, 64)
	libraryRevision, libraryRevisionErr := strconv.ParseInt(libraryRevisionValues[0], 10, 64)
	if entryErr != nil || entryID <= 0 || revisionErr != nil || revision < 0 || libraryRevisionErr != nil || libraryRevision < 1 {
		http.Error(w, "Review the current question and selected example", http.StatusBadRequest)
		return
	}
	job, err := a.jobByID(jobID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 {
		http.Error(w, "Completed applications cannot be edited", http.StatusConflict)
		return
	}
	destinationValues := r.PostForm["destination"]
	if len(destinationValues) != 1 || destinationValues[0] != job.URL {
		http.Error(w, "The application destination changed. Review the current question before using previous writing.", http.StatusConflict)
		return
	}
	var currentField schema.Field
	validCurrent := false
	if r.PostForm.Get("early") == "yes" {
		source, fp, sourceID := r.PostForm.Get("discovery_source"), r.PostForm.Get("discovery_fingerprint"), r.PostForm.Get("source_question_id")
		if source == "" || fp == "" || sourceID == "" || len(r.PostForm["early"]) != 1 || len(r.PostForm["discovery_source"]) != 1 || len(r.PostForm["discovery_fingerprint"]) != 1 || len(r.PostForm["source_question_id"]) != 1 {
			http.Error(w, "The discovered question changed. Review it before using previous writing.", http.StatusConflict)
			return
		}
		prepared, prepErr := a.prepareEarlyQuestions(job, r)
		if prepErr != nil || prepared.Item == nil || prepared.Item.Source != source || prepared.Item.Fingerprint != fp {
			http.Error(w, "The discovered question changed. Review it before using previous writing.", http.StatusConflict)
			return
		}
		for _, q := range prepared.Questions {
			if q.Key == keyValues[0] && q.SourceQuestionID == sourceID && q.Destination == job.URL && q.Revision == revision && q.Editable && q.TriageCategory == "answer" && earlyPotentialNarrative(q.Field) {
				currentField, validCurrent = q.Field, true
				break
			}
		}
	} else {
		profile, profileErr := a.candidateProfile()
		if profileErr != nil {
			http.Error(w, "Unable to read current application questions", http.StatusInternalServerError)
			return
		}
		prepared, prepErr := a.prepareQuestions(job, profile)
		if prepErr != nil {
			http.Error(w, "Unable to read current application questions", http.StatusInternalServerError)
			return
		}
		for _, q := range prepared.Questions {
			if q.Key == keyValues[0] && q.Destination == job.URL && q.Editable && q.Revision == revision && answerLibraryEligible(q.Field, true) {
				currentField, validCurrent = q.Field, true
				break
			}
		}
	}
	if !validCurrent || !answerLibraryEligible(currentField, true) {
		http.Error(w, "The current application question changed. Review it before using previous writing.", http.StatusConflict)
		return
	}
	entry, err := a.answerLibraryBySourceID(a.db, entryID)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Unable to read the selected Answer Library example", http.StatusInternalServerError)
		return
	}
	if entry.Revision != libraryRevision {
		http.Error(w, "This Answer Library example changed. Review the updated example before using it.", http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	_ = json.NewEncoder(w).Encode(struct {
		EntryID         int64  `json:"entry_id"`
		LibraryRevision int64  `json:"library_revision"`
		QuestionKey     string `json:"question_key"`
		Revision        int64  `json:"revision"`
		Answer          string `json:"answer"`
	}{entry.ID, entry.Revision, keyValues[0], revision, entry.Answer})
}

func (a *App) promoteApplicationAnswerHandler(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	keyValues, revisionValues := r.PostForm["question_key"], r.PostForm["revision"]
	if len(keyValues) != 1 || len(revisionValues) != 1 || !questionReturnKey.MatchString(keyValues[0]) {
		http.Error(w, "Review the saved answer before adding it to the Answer Library", http.StatusBadRequest)
		return
	}
	revision, err := strconv.ParseInt(revisionValues[0], 10, 64)
	if err != nil || revision < 1 {
		http.Error(w, "Review the saved answer revision before adding it to the Answer Library", http.StatusBadRequest)
		return
	}
	entry, created, err := a.promoteApplicationAnswer(id, keyValues[0], revision)
	if err != nil {
		switch {
		case errors.Is(err, ErrAnswerLibraryStale):
			http.Error(w, "This saved answer changed. Review its current revision before adding it to the Answer Library.", http.StatusConflict)
		case errors.Is(err, ErrAnswerLibraryIneligible):
			http.Error(w, "This question is unsupported for the Answer Library. Only verified, eligible narrative answers can be saved.", http.StatusUnprocessableEntity)
		case errors.Is(err, ErrAnswerLibrarySource):
			http.Error(w, "The saved source answer could not be verified. Review the current observed question and saved answer.", http.StatusConflict)
		default:
			http.Error(w, "Unable to save this example to the Answer Library", http.StatusInternalServerError)
		}
		return
	}
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "private, no-store")
		_ = json.NewEncoder(w).Encode(struct {
			Saved    bool  `json:"saved"`
			Existing bool  `json:"existing"`
			EntryID  int64 `json:"entryId"`
			Revision int64 `json:"revision"`
		}{true, !created, entry.ID, entry.Revision})
		return
	}
	notice := "saved"
	if !created {
		notice = "existing"
	}
	http.Redirect(w, r, "/answer-library?notice="+notice, http.StatusSeeOther)
}

func (a *App) promoteEarlyApplicationAnswerHandler(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	keyValues, revisionValues := r.PostForm["question_key"], r.PostForm["revision"]
	if len(keyValues) != 1 || len(revisionValues) != 1 || !questionReturnKey.MatchString(keyValues[0]) {
		http.Error(w, "Review the saved early answer before adding it to the Answer Library", http.StatusBadRequest)
		return
	}
	revision, err := strconv.ParseInt(revisionValues[0], 10, 64)
	if err != nil || revision < 1 {
		http.Error(w, "Review the saved early answer revision before adding it to the Answer Library", http.StatusBadRequest)
		return
	}
	entry, created, err := a.promoteEarlyApplicationAnswer(id, keyValues[0], revision)
	if err != nil {
		switch {
		case errors.Is(err, ErrAnswerLibraryStale):
			http.Error(w, "This saved early answer changed. Review its current revision before adding it to the Answer Library.", http.StatusConflict)
		case errors.Is(err, ErrAnswerLibraryIneligible):
			http.Error(w, "This early question is unsupported for the Answer Library. Only eligible saved narratives can be curated.", http.StatusUnprocessableEntity)
		case errors.Is(err, ErrAnswerLibrarySource):
			http.Error(w, "The saved early answer could not be verified. Review the current discovered question and saved answer.", http.StatusConflict)
		default:
			http.Error(w, "Unable to save this early example to the Answer Library", http.StatusInternalServerError)
		}
		return
	}
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "private, no-store")
		_ = json.NewEncoder(w).Encode(struct {
			Saved    bool  `json:"saved"`
			Existing bool  `json:"existing"`
			EntryID  int64 `json:"entryId"`
			Revision int64 `json:"revision"`
		}{true, !created, entry.ID, entry.Revision})
		return
	}
	notice := "saved"
	if !created {
		notice = "existing"
	}
	http.Redirect(w, r, "/answer-library?notice="+notice, http.StatusSeeOther)
}

type answerLibraryBrowseJSON struct {
	ID         int64  `json:"id"`
	Revision   int64  `json:"revision"`
	Question   string `json:"question"`
	Employer   string `json:"employer"`
	JobTitle   string `json:"jobTitle"`
	Host       string `json:"host"`
	Excerpt    string `json:"excerpt"`
	SourceKind string `json:"sourceKind"`
	UpdatedAt  string `json:"updatedAt"`
}

// GET /api/answer-library browses retained curated examples with bounded
// paging, independent of ranked top-3/latest-5,000 limits. With ?query= it
// delegates to the existing ranker and explicitly labels bounded results.
func (a *App) answerLibraryBrowseAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 1 && parsed <= 50 {
			limit = parsed
		} else if err == nil {
			http.Error(w, "Browse limit must be between 1 and 50", http.StatusBadRequest)
			return
		}
	}
	cursor := int64(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("cursor")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			http.Error(w, "Invalid browse cursor", http.StatusBadRequest)
			return
		}
		cursor = parsed
	}
	var libraryTotal int64
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM answer_library`).Scan(&libraryTotal); err != nil {
		http.Error(w, "Unable to browse saved writing", http.StatusInternalServerError)
		return
	}
	if query != "" {
		if len(query) > 1000 || !utf8.ValidString(query) || strings.ContainsRune(query, 0) {
			http.Error(w, "Search text is invalid or too long", http.StatusBadRequest)
			return
		}
		results, err := a.searchAnswerLibraryPaged(query, limit)
		if err != nil {
			http.Error(w, "Unable to search saved writing", http.StatusInternalServerError)
			return
		}
		entries := make([]answerLibraryBrowseJSON, 0, len(results))
		for _, item := range results {
			kind := "live"
			updated := ""
			_ = a.db.QueryRow(`SELECT COALESCE(source_kind,'live'),updated_at FROM answer_library WHERE id=?`, item.EntryID).Scan(&kind, &updated)
			if kind == "" {
				kind = "live"
			}
			entries = append(entries, answerLibraryBrowseJSON{
				ID: item.EntryID, Revision: item.Revision, Question: item.OriginalQuestion,
				Employer: item.SourceEmployer, JobTitle: item.SourceJobTitle, Host: item.SourceHost,
				Excerpt: item.Excerpt, SourceKind: kind, UpdatedAt: updated,
			})
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "private, no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"entries": entries, "nextCursor": 0, "hasMore": false,
			"libraryEmpty": libraryTotal == 0, "query": query,
			"bounded": true, "bound": answerLibraryLibrarySearchLimit,
		})
		return
	}
	rows, err := a.db.Query(`SELECT `+answerLibrarySelectColumns+` FROM answer_library WHERE (?=0 OR id<?)
	 ORDER BY id DESC LIMIT ?`, cursor, cursor, limit+1)
	if err != nil {
		http.Error(w, "Unable to browse saved writing", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var entries []answerLibraryBrowseJSON
	var lastID int64
	for rows.Next() {
		var entry AnswerLibraryEntry
		if err := scanAnswerLibraryEntry(rows, &entry); err != nil {
			rows.Close()
			http.Error(w, "Unable to browse saved writing", http.StatusInternalServerError)
			return
		}
		kind := entry.SourceKind
		if kind == "" {
			kind = "live"
		}
		entries = append(entries, answerLibraryBrowseJSON{
			ID: entry.ID, Revision: entry.Revision, Question: entry.OriginalQuestion,
			Employer: entry.SourceEmployer, JobTitle: entry.SourceJobTitle, Host: entry.SourceHost,
			Excerpt: answerLibraryExcerpt(entry.Answer), SourceKind: kind, UpdatedAt: entry.UpdatedAt,
		})
		lastID = entry.ID
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Unable to browse saved writing", http.StatusInternalServerError)
		return
	}
	hasMore := len(entries) > limit
	if hasMore {
		entries = entries[:limit]
		lastID = entries[len(entries)-1].ID
	}
	if entries == nil {
		entries = []answerLibraryBrowseJSON{}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"entries": entries, "nextCursor": lastID, "hasMore": hasMore,
		"libraryEmpty": libraryTotal == 0, "query": "", "bounded": false,
	})
}

func (a *App) editAnswerLibraryHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 24<<10) {
		return
	}
	revisionValues, answerValues := r.PostForm["revision"], r.PostForm["answer"]
	if len(revisionValues) != 1 || len(answerValues) != 1 {
		http.Error(w, "Invalid Answer Library edit", http.StatusBadRequest)
		return
	}
	revision, err := strconv.ParseInt(revisionValues[0], 10, 64)
	if err != nil || revision < 1 {
		http.Error(w, "Invalid Answer Library revision", http.StatusBadRequest)
		return
	}
	if err := a.updateAnswerLibraryWithAlternatives(id, revision, answerValues[0], strings.Split(r.PostForm.Get("alternatives"), "\n")); err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			http.NotFound(w, r)
		case errors.Is(err, ErrAnswerLibraryStale):
			http.Error(w, "This library copy changed in another window. Reload the Answer Library before editing.", http.StatusConflict)
		case errors.Is(err, ErrAnswerLibraryInvalid):
			http.Error(w, "Answer text must be valid UTF-8, nonempty, and at most 12,000 bytes", http.StatusUnprocessableEntity)
		default:
			http.Error(w, "Unable to save the Answer Library edit", http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/answer-library?notice=edited", http.StatusSeeOther)
}

func (a *App) deleteAnswerLibraryHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 2<<10) {
		return
	}
	if r.PostForm.Get("confirm") != "yes" {
		http.Error(w, "Check the confirmation to delete this Answer Library example", http.StatusBadRequest)
		return
	}
	revisionValues := r.PostForm["revision"]
	if len(revisionValues) != 1 {
		http.Error(w, "Review the current Answer Library revision before deleting this example", http.StatusBadRequest)
		return
	}
	revision, err := strconv.ParseInt(revisionValues[0], 10, 64)
	if err != nil || revision < 1 {
		http.Error(w, "Invalid Answer Library revision", http.StatusBadRequest)
		return
	}
	if err := a.deleteAnswerLibrary(id, revision); err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			http.NotFound(w, r)
		case errors.Is(err, ErrAnswerLibraryStale):
			http.Error(w, "This library copy changed in another window. Reload the Answer Library before deleting.", http.StatusConflict)
		case errors.Is(err, ErrAnswerLibraryInvalid):
			http.Error(w, "Invalid Answer Library revision", http.StatusBadRequest)
		default:
			http.Error(w, "Unable to delete this Answer Library example", http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/answer-library?notice=deleted", http.StatusSeeOther)
}
