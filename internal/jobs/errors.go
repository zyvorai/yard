package jobs

import "errors"

var (
	errNoPolicy   = errors.New("egress policy unavailable")
	errFetch      = errors.New("playbook fetch failed")
	errPeerStatus = errors.New("peer rejected events")
)
