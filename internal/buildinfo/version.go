// Package buildinfo holds the binary version injected at link time.
package buildinfo

// Version is set with -ldflags "-X quality_control/internal/buildinfo.Version=...".
var Version = "dev"
