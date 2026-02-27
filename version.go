package sconfig

var (
	// Version is the semantic-like version string of the library or consuming
	// application embedding this package. The format is "Major.Minor.Patch.Build".
	Version = "2.0.1.43" // Major, Minor, Patch, Build

	// BuildTime is the timestamp of the build, typically injected via -ldflags
	// at build time.
	BuildTime = "2026-02-27 11:28:46"
)
