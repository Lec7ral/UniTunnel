module unitunnel

// Wails v3.0.0-alpha.8 requires go >= 1.22.4. Keep this at the minimum so a
// clean clone builds with the toolchain pinned in CI (setup-go reads this
// file). Do not bump it to a version the local/CI toolchain cannot download.
go 1.22.4

require (
	github.com/wailsapp/wails/v3 v3.0.0-alpha.8
	golang.org/x/sys v0.21.0
)
