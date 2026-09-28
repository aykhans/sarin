package sarin

import (
	"math"
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

func safeUintToInt(u uint) int {
	if u > math.MaxInt {
		return math.MaxInt
	}
	return int(u)
}

func safeUint64ToInt(u uint64) int {
	if u > math.MaxInt {
		return math.MaxInt
	}
	return int(u)
}

func safeInt64ToInt(i int64) int {
	if i > math.MaxInt {
		return math.MaxInt
	}
	return int(i)
}
