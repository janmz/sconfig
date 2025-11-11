package sconfig

var (
	// Version is the semantic-like version string of the library or consuming
	// application embedding this package. The format is "Major.Minor.Branch.Build".
	Version = "1.0.1.1" // Major, Minor, Branch, Build

	// BuildTime is the timestamp of the build, typically injected via -ldflags
	// at build time.
	BuildTime = "2025-11-11 16:41:06"
)
