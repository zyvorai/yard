package egress

import "crypto/tls"

func insecureTLSConfig() *tls.Config {
	return &tls.Config{InsecureSkipVerify: true} //nolint:gosec // explicit lab flag only
}
