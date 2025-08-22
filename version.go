package sconfig

var (
	// Version is the semantic-like version string of the library or consuming
	// application embedding this package. The format is "Major.Minor.Branch.Build".
	Version = "1.0.0.0" // Major, Minor, Branch, Build

	// BuildTime is the timestamp of the build, typically injected via -ldflags
	// at build time.
	BuildTime = "2025-08-16 07:34:37"

	// GitCommit is the short commit hash of the build, typically injected via
	// -ldflags at build time.
	GitCommit = "unknown"
)
