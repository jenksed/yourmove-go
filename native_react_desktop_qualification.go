//go:build desktop && darwin && qualification

package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jenksed/yourmove-go/internal/app"
	"github.com/jenksed/yourmove-go/internal/storage"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// This executable exists only for an explicitly marked synthetic workspace.
// The receipt is local fixture evidence and never reaches an employer.
func main() {
	root := os.Getenv("YOURMOVE_DATA_DIR")
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || !strings.HasPrefix(filepath.Base(root), "yourmove-react-native-") {
		log.Fatal("invalid synthetic workspace")
	}
	marker, err := os.ReadFile(filepath.Join(root, ".yourmove-react-fixture"))
	if err != nil || string(marker) != "yourmove-react-fixture-v1\n" {
		log.Fatal("fixture marker required")
	}
	// ApplicationStorage can import a legacy jobs.db from the current working
	// directory. Pin cwd to this marked directory before it resolves paths.
	if err := os.Chdir(root); err != nil {
		log.Fatal(err)
	}
	if err := writeFixtureResume(filepath.Join(root, "synthetic-resume.docx"), "Synthetic Applicant — Product Engineer. Qualification data only."); err != nil {
		log.Fatal(err)
	}
	if err := writeFixtureResume(filepath.Join(root, "synthetic-letter-source.docx"), "Synthetic letter source for qualification only."); err != nil {
		log.Fatal(err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	var mu sync.Mutex
	receiptPath := filepath.Join(root, "synthetic-submission.json")
	var receipt struct {
		Count int `json:"count"`
	}
	if data, err := os.ReadFile(receiptPath); err == nil {
		if json.Unmarshal(data, &receipt) != nil {
			log.Fatal("invalid fixture receipt")
		}
	} else if !os.IsNotExist(err) {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /synthetic/application", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, fixturePage)
	})
	mux.HandleFunc("GET /synthetic/receipt", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(receipt)
	})
	mux.HandleFunc("POST /synthetic/receipt", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		receipt.Count++
		data, _ := json.Marshal(receipt)
		if err := os.WriteFile(receiptPath, data, 0600); err != nil {
			receipt.Count--
			http.Error(w, "receipt failed", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	})
	server := &http.Server{Handler: mux}
	defer server.Close()
	go func() { _ = server.Serve(listener) }()
	fixture := "http://" + listener.Addr().String() + "/synthetic"
	dbPath, resumeDir, err := storage.ApplicationStorage()
	if err != nil {
		log.Fatal(err)
	}
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	a, err := app.New(app.Config{DB: db, ResumeDir: resumeDir, TemplateFiles: templateFiles, StaticFiles: staticFiles, FrontendDist: frontendDist, CoverLetterInstructions: coverLetterInstructions, ApplicationAnswerInstructions: applicationAnswerInstructions,
		PickResume: func() (string, error) {
			return application.Get().Dialog.OpenFile().SetTitle("Select a synthetic résumé").AddFilter("Resumes", "*.pdf;*.docx").PromptForSingleSelection()
		},
		ChooseExportPath: func(name string) (string, error) {
			return application.Get().Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{Title: "Save a copy", Filename: name, CanCreateDirectories: true}).PromptForSingleSelection()
		},
		OpenPosting: func(string) error { return fmt.Errorf("external posting disabled in qualification") },
	})
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()
	if err := app.ConfigureNativeReactFixture(a, root, fixture); err != nil {
		log.Fatal(err)
	}
	log.Printf("SYNTHETIC ONLY workspace=%s resume=%s fixture=%s/application receipt=%s", root, filepath.Join(root, "synthetic-resume.docx"), fixture, receiptPath)
	desktop := application.New(application.Options{Name: "YourMove React Qualification", Assets: application.AssetOptions{Handler: a.DesktopHandler()}})
	desktop.Window.NewWithOptions(application.WebviewWindowOptions{Title: "YourMove — Synthetic Qualification", Width: 1100, Height: 800, MinWidth: 700, MinHeight: 550, URL: "/jobs"})
	if err := desktop.Run(); err != nil {
		log.Fatal(err)
	}
}

func writeFixtureResume(path, text string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	f, err := z.Create("[Content_Types].xml")
	if err != nil {
		return err
	}
	_, _ = fmt.Fprint(f, `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/></Types>`)
	f, err = z.Create("word/document.xml")
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(f, `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>%s</w:t></w:r></w:p></w:body></w:document>`, text)
	if err := z.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0600)
}

const fixturePage = `<!doctype html><html><head><meta charset="utf-8"><title>Synthetic employer form</title><style>body{font:16px system-ui;max-width:720px;margin:3rem auto;padding:0 1rem}label{display:block;margin:1rem 0}input,textarea{display:block;width:100%;padding:.5rem}button{padding:.7rem}.status{background:#eef5f0;padding:1rem}</style></head><body><h1>Synthetic employer application</h1><p>Loopback fixture only. No real employer is contacted.</p><div class="status" role="status">Local submissions: <span id="count">loading</span><br><span id="receipt">No manual submission this session</span></div><div class="ashby-application-form-container"><form class="ashby-application-form"><label>First name<input name="first_name" autocomplete="given-name"></label><label>Last name<input name="last_name" autocomplete="family-name"></label><label>Email<input type="email" name="email" autocomplete="email"></label><section data-field-entry-id="resume"><div class="ashby-application-form-question-title">Resume</div><input type="file" name="resume" accept=".docx,.pdf"></section><section data-field-entry-id="cover"><div class="ashby-application-form-question-title">Cover Letter</div><input type="file" name="cover" accept=".pdf"></section><label>Why this team?<textarea name="motivation"></textarea></label><button type="submit">Submit synthetic application</button></form></div><script>const count=document.querySelector('#count');fetch('/synthetic/receipt').then(r=>r.json()).then(x=>count.textContent=x.count);document.querySelector('form').addEventListener('submit',async e=>{e.preventDefault();const r=await fetch('/synthetic/receipt',{method:'POST'});if(!r.ok){document.querySelector('#receipt').textContent='Local receipt failed';return}const x=await r.json();count.textContent=x.count;document.querySelector('#receipt').textContent='Synthetic submission received locally — receipt '+x.count})</script></body></html>`
