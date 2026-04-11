package sarin

import (
	"math/rand/v2"
	"time"
)

func NewDefaultRandSource() rand.Source {
	now := time.Now().UnixNano()
	return rand.NewPCG(
		uint64(now),
		uint64(now>>32),
	)
}

func firstOrEmpty(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
