//go:build desktop && windows && !qualification

package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/jenksed/yourmove-go/internal/app"
)

// Windows updater hooks are intentional no-ops. No Windows updater behavior
// is in scope; the manual install path remains the fallback.
func configureDesktopUpdater(_ *app.App, _ *application.App) {}

func startDesktopUpdateTasks(_ *app.App, _ *application.App) {}
