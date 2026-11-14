package app

// Single build-time/release-time version source for the installed
// application. It must stay coherent with release/macos/Info.plist
// (CFBundleShortVersionString / CFBundleVersion) and is the CurrentVersion
// handed to the Wails updater. Do not scatter hardcoded versions elsewhere.
//
// Vars (not consts) so release candidates can pin an exact version via:
//   go build -ldflags "-X github.com/jenksed/yourmove-go/internal/app.AppVersion=0.2.0 -X github.com/jenksed/yourmove-go/internal/app.AppBuild=5"
var (
	AppVersion = "0.2.0"
	AppBuild   = "5"
)

// CurrentAppVersion returns the version the updater advertises as installed.
func CurrentAppVersion() string { return AppVersion }

// CurrentAppBuild returns the Info.plist CFBundleVersion counterpart.
func CurrentAppBuild() string { return AppBuild }
