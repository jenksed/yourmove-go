//go:build desktop && darwin

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

const probePage = `<!doctype html><html lang="en"><head><meta charset="utf-8">
<title>YourMove WebView body probe</title></head><body>
<h1>Wails request-body probe</h1>
<p>Diagnostic only. No uploaded file leaves this local desktop process.</p>
<button id="run">Run body probes</button><pre id="result">Not run</pre>
<script>
const target = document.getElementById('result');
document.getElementById('run').onclick = async () => {
  target.textContent = 'Running…';
  const results = {};
  const requests = [];
  requests.push(['urlencoded', new URLSearchParams({phrase: 'A&B = + é'})]);
  const form = new FormData();
  form.append('note', 'body-probe');
  form.append('blob', new Blob(['yourmove-blob-probe'], {type: 'text/plain'}), 'probe.txt');
  requests.push(['multipart-blob', form]);
  for (const [name, body] of requests) {
    try {
      const response = await fetch('/__wails_probe', {method: 'POST', body});
      results[name] = {http_status: response.status, ...(await response.json())};
    } catch (error) { results[name] = {transport_error: String(error)}; }
  }
  target.textContent = JSON.stringify(results, null, 2);
};
</script></body></html>`

func showProbe(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, probePage)
}

// The probe returns measurements, not the supplied form contents.
func receiveProbe(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	result := map[string]any{"method": r.Method, "content_type": r.Header.Get("Content-Type")}
	contentType := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(contentType, "application/x-www-form-urlencoded"):
		err := r.ParseForm()
		result["urlencoded_ok"] = err == nil && r.PostForm.Get("phrase") == "A&B = + é"
		if err != nil {
			result["parse_error"] = err.Error()
		}
	case strings.HasPrefix(contentType, "multipart/form-data"):
		err := r.ParseMultipartForm(64 << 10)
		if err != nil {
			result["parse_error"] = err.Error()
		}
		if err == nil {
			file, _, err := r.FormFile("blob")
			if err != nil {
				result["parse_error"] = err.Error()
			} else {
				defer file.Close()
				bytes, err := io.ReadAll(io.LimitReader(file, 64<<10))
				result["blob_bytes"] = len(bytes)
				result["blob_ok"] = err == nil && string(bytes) == "yourmove-blob-probe"
			}
		}
	default:
		result["parse_error"] = "unexpected content type"
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
