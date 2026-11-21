package sconfig

var (
	// Version is the semantic-like version string of the library or consuming
	// application embedding this package. The format is "Major.Minor.Patch.Build".
	Version = "3.1.0.66" // Major, Minor, Patch, Build

	// BuildTime is the timestamp of the build, typically injected via -ldflags
	// at build time.
	BuildTime = "2026-11-21 13:05:51"
)
