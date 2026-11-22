// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0

package jobs

import "errors"

var (
	errNoPolicy   = errors.New("egress policy unavailable")
	errFetch      = errors.New("playbook fetch failed")
	errPeerStatus = errors.New("peer rejected events")
)
