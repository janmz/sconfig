package sconfig

var (
	// Version is the semantic-like version string of the library or consuming
	// application embedding this package. The format is "Major.Minor.Patch.Build".
	Version = "1.2.6.15" // Major, Minor, Patch, Build

	// BuildTime is the timestamp of the build, typically injected via -ldflags
	// at build time.
	BuildTime = "2025-12-03 17:03:41"
)
