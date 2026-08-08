package sarin

import (
	"context"
	"net"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"go.aykhans.me/sarin/internal/types"
)

// BenchmarkRealLoad drives a full run against a server in a *separate process*,
// so a CPU profile of this benchmark contains only client-side work.
//
// Start the server first (any HTTP server will do) and point the benchmark at it:
//
//	SARIN_BENCH_ADDR=127.0.0.1:19090 go test -run XXX -bench RealLoad -cpuprofile cpu.out
//
// Without SARIN_BENCH_ADDR the benchmark is skipped.
func BenchmarkRealLoad(b *testing.B) {
	addr := os.Getenv("SARIN_BENCH_ADDR")
	if addr == "" {
		b.Skip("set SARIN_BENCH_ADDR to run")
	}
	if c, err := net.DialTimeout("tcp", addr, time.Second); err != nil {
		b.Skipf("no server at %s: %v", addr, err)
	} else {
		_ = c.Close()
	}

	workers, err := strconv.ParseUint(envOr("SARIN_BENCH_WORKERS", "128"), 10, 32)
	if err != nil {
		b.Fatal(err)
	}
	collectStats := os.Getenv("SARIN_BENCH_NOSTATS") == ""

	requestURL, err := url.Parse("http://" + addr + "/api")
	if err != nil {
		b.Fatal(err)
	}

	total := uint64(b.N)
	srn, err := NewSarin(
		b.Context(),
		[]string{"GET"}, requestURL, 5*time.Second, uint(workers), &total, nil,
		false, false,
		nil,
		types.Headers{{Key: "User-Agent", Value: []string{"sarin"}}},
		nil, nil, nil, nil,
		collectStats,
		false,
		"", "", nil, nil,
	)
	if err != nil {
		b.Fatal(err)
	}

	ctx, cancel := context.WithCancel(b.Context())
	defer cancel()

	b.ResetTimer()
	srn.Start(ctx, NewStopController(cancel))
	b.StopTimer()
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
