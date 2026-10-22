package chromium

import (
	"errors"
	"testing"
)

func TestSelectTargetKeepsPostingIdentityAndPrefersApplication(t *testing.T) {
	saved := "https://jobs.ashbyhq.com/render/e1f2d6a2-b9e2-4f16-b863-3ca228c83e75"
	posting := target{Type: "page", URL: saved, WebSocketURL: "ws://localhost:4567/devtools/page/a"}
	other := target{Type: "page", URL: "https://jobs.ashbyhq.com/else/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/application", WebSocketURL: "ws://127.0.0.1:4567/devtools/page/b"}
	app := target{Type: "page", URL: saved + "/application", WebSocketURL: "ws://localhost:4567/devtools/page/c"}
	chosen, err := selectTarget(saved, 4567, []target{other, posting, app})
	if err != nil || chosen.URL != app.URL || chosen.WebSocketURL != "ws://127.0.0.1:4567/devtools/page/c" {
		t.Fatalf("chose %+v: %v", chosen, err)
	}
	fallback, err := selectTarget(saved, 4567, []target{other, posting})
	if err != nil || fallback.URL != saved {
		t.Fatalf("bad fallback %+v %v", fallback, err)
	}
	for _, ws := range []string{"ws://evil.example:4567/devtools/page/c", "ws://localhost:9999/devtools/page/c", "wss://localhost:4567/devtools/page/c"} {
		if _, err := selectTarget(saved, 4567, []target{{Type: "page", URL: app.URL, WebSocketURL: ws}}); err == nil {
			t.Errorf("accepted unsafe websocket %q", ws)
		}
	}
}

func TestMissingTargetIsARecoverableBrowserState(t *testing.T) {
	saved := "https://jobs.ashbyhq.com/render/e1f2d6a2-b9e2-4f16-b863-3ca228c83e75"
	if _, err := selectTarget(saved, 4567, nil); !errors.Is(err, ErrNoPageTargets) {
		t.Fatalf("no pages should be recognized: %v", err)
	}
	auth := target{Type: "page", URL: "https://auth.example.com", WebSocketURL: "ws://localhost:4567/devtools/page/auth"}
	if _, err := selectTarget(saved, 4567, []target{auth}); !errors.Is(err, ErrNoMatchingTab) {
		t.Fatalf("foreign login should not imply closed browser: %v", err)
	}
}

func TestLaunchArgumentsUseApplicationNotDescription(t *testing.T) {
	posting := "https://jobs.ashbyhq.com/render/e1f2d6a2-b9e2-4f16-b863-3ca228c83e75"
	apply := posting + "/application"
	args := launchArguments("/tmp/isolated-profile", apply)
	for _, arg := range args {
		if arg == "--app="+posting {
			t.Fatal("launched at job description instead of application")
		}
		if arg == "--app="+apply {
			return
		}
	}
	t.Fatalf("missing direct application landing: %v", args)
}

func TestPreviewLaunchArgumentsUseAnIsolatedHeadlessProfile(t *testing.T) {
	posting := "https://jobs.ashbyhq.com/render/e1f2d6a2-b9e2-4f16-b863-3ca228c83e75"
	args := previewLaunchArguments("/tmp/isolated-preview-profile", posting+"/application")
	want := map[string]bool{
		"--user-data-dir=/tmp/isolated-preview-profile": true,
		"--app=" + posting + "/application":             true,
		"--headless=new":                                true,
	}
	for _, arg := range args {
		delete(want, arg)
	}
	if len(want) != 0 {
		t.Fatalf("preview is missing isolated/headless launch arguments: %v in %v", want, args)
	}
}
