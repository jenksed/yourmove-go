package app

// Single build-time/release-time version source for the installed
// application. It must stay coherent with release/macos/Info.plist
// (CFBundleShortVersionString / CFBundleVersion) and is the CurrentVersion
// handed to the Wails updater. Do not scatter hardcoded versions elsewhere.
const (
	AppVersion = "0.1.0"
	AppBuild   = "3"
)

// CurrentAppVersion returns the version the updater advertises as installed.
func CurrentAppVersion() string { return AppVersion }

// CurrentAppBuild returns the Info.plist CFBundleVersion counterpart.
func CurrentAppBuild() string { return AppBuild }
