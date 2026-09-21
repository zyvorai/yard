package version

// Version is reported on /metrics as yard_build_info. Override with
// -ldflags "-X github.com/zyvorai/yard/internal/version.Version=..."
var Version = "dev"
