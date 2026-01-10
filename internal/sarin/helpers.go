package sarin

import (
	"math/rand/v2"
	"time"
)

func NewDefaultRandSource() rand.Source {
	now := time.Now().UnixNano()
	return rand.NewPCG(
		uint64(now),     //nolint:gosec // G115: Safe conversion; UnixNano timestamp used as random seed, bit pattern is intentional
		uint64(now>>32), //nolint:gosec // G115: Safe conversion; right-shifted timestamp for seed entropy, overflow is acceptable
	)
}
