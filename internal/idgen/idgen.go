package idgen

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func New(prefix string) string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s_%s%s", prefix, strings.ToLower(time.Now().UTC().Format("060102")), hex.EncodeToString(b[:]))
}

func Secret(n int) string {
	if n < 16 {
		n = 16
	}
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
