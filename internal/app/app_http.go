package app

import (
	"context"
	"database/sql"
	"github.com/jenksed/yourmove-go/internal/application/schema"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

type App struct {
	answerAI                  answerAIRegistry
	applicationAnswerGenerate func(context.Context, string, string) (string, error)
	careerDiscovery           careerDiscoveryGuard
	careerDiscoveryGenerate   func(context.Context, string, string) (string, error) // test override; production uses the configured provider
	careerStudio              careerDiscoveryGuard
	careerStudioGenerate      func(context.Context, string, string) (string, error) // test override; production uses the configured provider
	resources                 *appResources
	intakeMu                  sync.Mutex
	enrichmentMu              sync.Mutex
	enrichment                *enrichmentRun
	enrichmentFetch          postingFetcher
	desktopUnsaved            atomic.Bool
	db                        *sql.DB
	resumeDir                 string
	pickResume                func() (string, error)
	chooseExportPath          func(string) (string, error) // trusted desktop Save dialog only
	openPosting               func(string) error           // desktop-only native default-browser action
	inspectApplication        func(context.Context, string) (schema.Result, error)
	previewApplication        func(context.Context, string) (schema.Result, error)
	earlyDiscoveryMu          sync.Mutex
	browsers                  sessionHost
	coverLetterGenerate       func(context.Context, string) (string, error) // test override; production uses local Ollama
	updates                   updateState
	updateCheck               func(context.Context) (UpdateCheckResult, error)
	updateInstall             func(context.Context) error
	updateRestart             func(context.Context) error
}

func newMux(app *App) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/",
		http.FileServer(http.FS(app.resources.staticFiles)))
	mux.HandleFunc("/", home)
	mux.HandleFunc("/jobs", app.serveOpportunities)
	mux.HandleFunc("POST /jobs/import", app.importJobs)
	mux.HandleFunc("POST /jobs/delete", app.deleteSelectedJobs)
	mux.Handle("/app/", http.StripPrefix("/app/", app.serveFrontendAssets()))
	mux.HandleFunc("GET /api/opportunities", app.opportunitiesAPI)
	mux.HandleFunc("POST /api/opportunities", app.opportunitiesAPI)
	mux.HandleFunc("POST /api/opportunities/import", app.opportunitiesImportAPI)
	mux.HandleFunc("POST /api/opportunities/delete", app.opportunitiesDeleteAPI)
	mux.HandleFunc("POST /api/opportunities/{id}/open", app.opportunityOpenAPI)
	mux.HandleFunc("GET /utilities", app.utilitiesPage)
	mux.HandleFunc("POST /utilities/browser", app.saveApplicationBrowser)
	mux.HandleFunc("POST /utilities/updates/check", app.updateCheckHandler)
	mux.HandleFunc("POST /utilities/updates/install", app.updateInstallHandler)
	mux.HandleFunc("POST /utilities/updates/restart", app.updateRestartHandler)
	mux.HandleFunc("GET /api/updates/status", app.updateStatusAPI)
	mux.HandleFunc("GET /ai", app.aiConnectionsPage)
	mux.HandleFunc("GET /candidate", app.candidatePage)
	mux.HandleFunc("GET /requirements", app.requirementsPage)
	mux.HandleFunc("GET /answer-library", app.answerLibrary)
	mux.HandleFunc("GET /answer-library/{id}", app.answerLibraryEntryHandler)
	mux.HandleFunc("GET /answer-library/{id}/content", app.answerLibraryEntryContentHandler)
	mux.HandleFunc("POST /answer-library/search", app.searchAnswerLibraryHandler)
	mux.HandleFunc("POST /answer-library/{id}/edit", app.editAnswerLibraryHandler)
	mux.HandleFunc("POST /answer-library/{id}/delete", app.deleteAnswerLibraryHandler)
	mux.HandleFunc("POST /answer-library/{id}/career-evidence", app.createCareerEvidenceFromLibraryHandler)
	mux.HandleFunc("GET /career-evidence", app.careerEvidencePage)
	mux.HandleFunc("POST /career-evidence", app.createCareerEvidenceHandler)
	mux.HandleFunc("GET /career-evidence/{id}", app.careerEvidenceEntryHandler)
	mux.HandleFunc("POST /career-evidence/{id}/save", app.saveCareerEvidenceHandler)
	mux.HandleFunc("POST /career-evidence/{id}/review", app.reviewCareerEvidenceHandler)
	mux.HandleFunc("POST /career-evidence/{id}/delete", app.deleteCareerEvidenceHandler)
	mux.HandleFunc("POST /candidate", app.updateCandidate)
	mux.HandleFunc("GET /families", app.familyPage)
	mux.HandleFunc("POST /families", app.createFamily)
	mux.HandleFunc("POST /families/examples", app.addExamples)
	mux.HandleFunc("POST /families/test", app.testFamilyMatch)
	mux.HandleFunc("POST /families/{id}/edit", app.editFamily)
	mux.HandleFunc("POST /families/{id}/delete", app.removeFamily)
	mux.HandleFunc("POST /resumes/{id}/family", app.setResumeFamily)
	mux.HandleFunc("POST /resumes/{id}/master", app.chooseMasterResume)
	mux.HandleFunc("POST /resumes/{id}/delete", app.deleteResumeHandler)
	mux.HandleFunc("GET /enrichment", app.enrichmentPage)
	mux.HandleFunc("POST /enrichment/{action}", app.enrichmentAction)
	mux.HandleFunc("GET /jobs/{id}", app.serveJobWorkspace)
	mux.HandleFunc("GET /jobs/{id}/brief", app.applicationBriefHandler)
	mux.HandleFunc("GET /jobs/{id}/inspection", app.inspectionPage)
	mux.HandleFunc("POST /jobs/{id}/inspect", app.inspectJob)
	mux.HandleFunc("POST /jobs/{id}/answers/preview", app.previewEarlyQuestions)
	mux.HandleFunc("POST /jobs/{id}/continue", app.continueApplication)
	mux.HandleFunc("POST /jobs/{id}/handoff", app.startResumeHandoff)
	mux.HandleFunc("POST /jobs/{id}/browser/{action}", app.browserOperation)
	mux.HandleFunc("GET /jobs/{id}/browser-status", app.browserStatus)
	mux.HandleFunc("POST /jobs/{id}/edit", app.editJob)
	mux.HandleFunc("POST /jobs/{id}/open", app.openJobPosting)
	mux.HandleFunc("POST /jobs/{id}/delete", app.deleteJobHandler)
	mux.HandleFunc("POST /jobs/{id}/refresh", app.refreshJob)
	mux.HandleFunc("POST /jobs/{id}/cover-letter/generate", app.generateCoverLetterHandler)
	mux.HandleFunc("POST /jobs/{id}/cover-letter/save", app.saveCoverLetterHandler)
	mux.HandleFunc("GET /jobs/{id}/cover-letter/{draft}/download", app.downloadCoverLetter)
	mux.HandleFunc("POST /jobs/{id}/cover-letter/transfer", app.transferCoverLetter)
	mux.HandleFunc("POST /jobs/{id}/answers/save", app.saveApplicationAnswer)
	mux.HandleFunc("POST /jobs/{id}/answers/early-save", app.saveEarlyApplicationAnswer)
	mux.HandleFunc("POST /jobs/{id}/answers/early-accept", app.acceptEarlyApplicationAnswer)
	mux.HandleFunc("POST /jobs/{id}/answers/ai/{action}", app.applicationAnswerAIHandler)
	mux.HandleFunc("POST /jobs/{id}/answers/help", app.applicationAnswerHelpHandler)
	mux.HandleFunc("POST /jobs/{id}/answers/library", app.promoteApplicationAnswerHandler)
	mux.HandleFunc("POST /jobs/{id}/answers/early-library", app.promoteEarlyApplicationAnswerHandler)
	mux.HandleFunc("POST /jobs/{id}/answers/starting-point", app.answerLibraryStartingPointHandler)
	mux.HandleFunc("GET /api/answer-library", app.answerLibraryBrowseAPI)
	mux.HandleFunc("POST /jobs/{id}/answers/transfer", app.transferApplicationAnswers)
	mux.HandleFunc("GET /api/jobs/{id}/workspace", app.workspaceAPI)
	mux.HandleFunc("POST /api/jobs/{id}/details", app.workspaceDetailsAPI)
	mux.HandleFunc("POST /api/jobs/{id}/refresh", app.workspaceRefreshAPI)
	mux.HandleFunc("GET /api/jobs/{id}/questions", app.writingQuestionsAPI)
	mux.HandleFunc("POST /api/jobs/{id}/questions/discover", app.writingDiscoverAPI)
	mux.HandleFunc("POST /api/jobs/{id}/answers", app.writingSaveAPI)
	mux.HandleFunc("POST /api/jobs/{id}/assistance", app.writingAssistAPI)
	mux.HandleFunc("POST /api/jobs/{id}/manual-questions", app.manualQuestionsCreateAPI)
	mux.HandleFunc("POST /api/jobs/{id}/manual-questions/{key}", app.manualQuestionSaveAPI)
	mux.HandleFunc("DELETE /api/jobs/{id}/manual-questions/{key}", app.manualQuestionDeleteAPI)
	mux.HandleFunc("POST /api/jobs/{id}/manual-questions/{key}/starting-point", app.manualQuestionStartingPointAPI)
	mux.HandleFunc("GET /api/jobs/{id}/letters", app.writingLettersAPI)
	mux.HandleFunc("POST /api/jobs/{id}/letters", app.writingLetterGenerateAPI)
	mux.HandleFunc("POST /api/jobs/{id}/letters/manual", app.writingLetterManualAPI)
	mux.HandleFunc("POST /api/jobs/{id}/letters/select", app.writingLetterSelectAPI)
	mux.HandleFunc("GET /api/jobs/{id}/letters/history", app.coverLetterHistoryAPI)
	mux.HandleFunc("GET /api/jobs/{id}/letters/{draft}", app.writingLetterAPI)
	mux.HandleFunc("POST /api/jobs/{id}/letters/{draft}", app.writingLetterSaveAPI)
	mux.HandleFunc("GET /api/cover-letters", app.coverLetterCatalogAPI)
	mux.HandleFunc("GET /api/cover-letters/{id}", app.coverLetterCatalogDetailAPI)
	mux.HandleFunc("GET /api/jobs/{id}/continuation", app.continuationAPI)
	mux.HandleFunc("POST /api/jobs/{id}/browser", app.continuationBrowserAPI)
	mux.HandleFunc("POST /api/jobs/{id}/continue", app.continuationContinueAPI)
	mux.HandleFunc("POST /api/jobs/{id}/submission-review", app.continuationReviewAPI)
	mux.HandleFunc("POST /api/jobs/{id}/submit", app.continuationSubmitAPI)
	mux.HandleFunc("POST /api/jobs/{id}/unsubmit", app.continuationUnsubmitAPI)
	mux.HandleFunc("GET /api/jobs/{id}/submissions", app.continuationSubmissionsAPI)
	mux.HandleFunc("POST /api/jobs/{id}/next", app.continuationNextAPI)
	mux.HandleFunc("POST /api/desktop/unsaved", app.desktopUnsavedAPI)
	mux.HandleFunc("GET /api/resumes", app.resumesAPI)
	mux.HandleFunc("GET /api/jobs/{id}/resume-options", app.resumeOptionsAPI)
	mux.HandleFunc("POST /api/jobs/{id}/resume", app.setJobResumeAPI)
	mux.HandleFunc("POST /api/resumes/import", app.importResumeAPI)
	mux.HandleFunc("POST /api/resumes/import-native", app.importResumeNativeAPI)
	mux.HandleFunc("POST /api/resumes/{id}/family", app.resumeFamilyAPI)
	mux.HandleFunc("POST /api/resumes/{id}/master", app.resumeMasterAPI)
	mux.HandleFunc("POST /api/resumes/{id}/archive", app.resumeArchiveAPI)
	mux.HandleFunc("POST /api/resumes/{id}/revise", app.resumeReviseAPI)
	mux.HandleFunc("POST /api/resumes/{id}/revise-native", app.resumeReviseNativeAPI)
	mux.HandleFunc("POST /api/resumes/{id}/delete", app.resumeDeleteAPI)
	mux.HandleFunc("GET /resumes", app.serveResumes)
	mux.HandleFunc("GET /career", app.serveCareerDiscovery)
	mux.HandleFunc("GET /career/studio", app.serveCareerDiscovery)
	mux.HandleFunc("GET /api/career/discovery/providers", app.careerDiscoveryProvidersAPI)
	mux.HandleFunc("GET /api/career/discovery/sessions", app.careerDiscoveryListAPI)
	mux.HandleFunc("POST /api/career/discovery/sessions", app.careerDiscoveryCreateAPI)
	mux.HandleFunc("GET /api/career/discovery/sessions/{id}", app.careerDiscoveryGetAPI)
	mux.HandleFunc("POST /api/career/discovery/sessions/{id}/authorize", app.careerDiscoveryAuthorizeAPI)
	mux.HandleFunc("POST /api/career/discovery/sessions/{id}/select", app.careerDiscoverySelectAPI)
	mux.HandleFunc("POST /api/career/discovery/sessions/{id}/answer", app.careerDiscoveryAnswerAPI)
	mux.HandleFunc("POST /api/career/discovery/sessions/{id}/promote", app.careerDiscoveryPromoteAPI)
	mux.HandleFunc("GET /api/career/evidence", app.careerEvidenceListAPI)
	mux.HandleFunc("POST /api/career/evidence", app.careerEvidenceCreateAPI)
	mux.HandleFunc("GET /api/career/evidence/{id}", app.careerEvidenceGetAPI)
	mux.HandleFunc("POST /api/career/evidence/{id}/save", app.careerEvidenceSaveHandlerAPI)
	mux.HandleFunc("POST /api/career/evidence/{id}/review", app.careerEvidenceReviewAPI)
	mux.HandleFunc("POST /api/career/evidence/{id}/reopen", app.careerEvidenceReopenAPI)
	mux.HandleFunc("POST /api/career/evidence/{id}/delete", app.careerEvidenceDeleteAPI)
	mux.HandleFunc("GET /api/career/roles", app.careerRolesAPI)
	mux.HandleFunc("GET /api/career/roles/{key}", app.careerRoleSnapshotAPI)
	mux.HandleFunc("POST /api/career/roles/{key}/authorize", app.careerRoleAuthorizeAPI)
	mux.HandleFunc("GET /api/career/synthesis", app.careerSynthesisAPI)
	mux.HandleFunc("POST /api/career/synthesis/authorize", app.careerSynthesisAuthorizeAPI)
	mux.HandleFunc("POST /api/career/synthesis/challenges/{id}", app.careerChallengeAPI)
	mux.HandleFunc("GET /api/career/activation", app.careerActivationAPI)
	mux.HandleFunc("GET /applications", app.applicationsPage)
	mux.HandleFunc("POST /resumes/import", app.uploadResume)
	mux.HandleFunc("POST /resumes/import-native", app.nativeResumeImport)
	mux.HandleFunc("GET /resumes/{id}/download", app.downloadResume)
	mux.HandleFunc("POST /jobs/{id}/resume", app.setJobResume)
	mux.HandleFunc("POST /jobs/{id}/submit", app.submitJob)
	mux.HandleFunc("POST /jobs/{id}/next", app.nextApplication)
	mux.HandleFunc("POST /resumes/{id}/archive", app.changeResumeArchive)
	mux.HandleFunc("POST /resumes/{id}/revise", app.reviseResumeBrowser)
	mux.HandleFunc("POST /resumes/{id}/revise-native", app.reviseResumeNative)

	return protectLocalRequests(mux)
}

// Protect reads as well as writes against unexpected hosts and browser origins.
// Comparing Origin to Host alone does not stop DNS rebinding: both can name
// the attacker's domain while the connection reaches our loopback listener.
func protectLocalRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !localRequestHost(r.Host) || r.Header.Get("Sec-Fetch-Site") == "cross-site" ||
			len(r.Header.Values("Origin")) > 1 {
			if strings.Contains(r.URL.Path, "/answers/ai/") {
				answerAIError(w, 403, "local_boundary")
			} else {
				http.Error(w, "Local application request required", http.StatusForbidden)
			}
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
			if strings.Contains(r.URL.Path, "/answers/ai/") {
				answerAIError(w, 403, "local_boundary")
			} else {
				http.Error(w, "Cross-origin requests are not allowed", http.StatusForbidden)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

func localRequestHost(host string) bool {
	u, err := url.Parse("http://" + host)
	if err != nil || u.Host != host || u.User != nil || u.Path != "" ||
		u.RawQuery != "" || u.Fragment != "" || strings.HasSuffix(host, ":") {
		return false
	}
	hostname := strings.ToLower(u.Hostname())
	switch hostname {
	case "localhost", "127.0.0.1", "::1":
	default:
		return false
	}
	if hostname == "::1" {
		hostname = "[::1]"
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n <= 0 || n > 65535 {
			return false
		}
		hostname += ":" + port
	}
	return strings.EqualFold(host, hostname)
}
