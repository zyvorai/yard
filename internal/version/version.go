// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0

package version

// Version is reported on /metrics as yard_build_info. Override with
// -ldflags "-X github.com/zyvorai/yard/internal/version.Version=..."
var Version = "dev"
