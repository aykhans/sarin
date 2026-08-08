package sarin

import (
	"context"
	"net"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
	"go.aykhans.me/sarin/internal/types"
)

// TestJobSourceHandsOutExactlyTotal checks that concurrent workers claiming from
// one job source between them get the requested number of jobs and no more.
func TestJobSourceHandsOutExactlyTotal(t *testing.T) {
	// Zero is not covered here: it means "no limit", which never drains. The stop
	// flag path covers it below.
	for _, total := range []uint64{1, 7, 5000} {
		for _, workers := range []int{1, 8, 64} {
			var stopped atomic.Bool
			requested := total
			jobs := newJobSource(&requested, &stopped)

			var claimed atomic.Uint64
			var wg sync.WaitGroup
			for range workers {
				wg.Go(func() {
					for jobs() {
						claimed.Add(1)
					}
				})
			}
			wg.Wait()

			if got := claimed.Load(); got != total {
				t.Errorf("total=%d workers=%d: claimed %d jobs, want %d", total, workers, got, total)
			}
		}
	}
}

// TestJobSourceStops checks that an unlimited source drains once the run is
// stopped, and that a counted source stops early too.
func TestJobSourceStops(t *testing.T) {
	t.Run("unlimited", func(t *testing.T) {
		var stopped atomic.Bool
		jobs := newJobSource(nil, &stopped)

		var claimed atomic.Uint64
		done := make(chan struct{})
		go func() {
			defer close(done)
			for jobs() {
				claimed.Add(1)
			}
		}()

		time.Sleep(20 * time.Millisecond)
		stopped.Store(true)

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("worker did not stop after the stop flag was set")
		}
		if claimed.Load() == 0 {
			t.Error("worker claimed no jobs before being stopped")
		}
	})

	t.Run("counted", func(t *testing.T) {
		var stopped atomic.Bool
		stopped.Store(true)
		total := uint64(1000)
		jobs := newJobSource(&total, &stopped)

		if jobs() {
			t.Error("job source handed out work while stopped")
		}
	})
}

// startTestServer serves 200 OK and counts the requests it received.
func startTestServer(t *testing.T) (addr string, received *atomic.Uint64) {
	t.Helper()

	received = &atomic.Uint64{}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	server := &fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			received.Add(1)
			ctx.SetStatusCode(200)
			ctx.SetBodyString("ok")
		},
		Concurrency: 10000,
	}
	go func() { _ = server.Serve(ln) }()
	t.Cleanup(func() { _ = server.Shutdown() })

	return ln.Addr().String(), received
}

func newTestSarin(t *testing.T, addr string, requests *uint64, duration *time.Duration, workers uint) *sarin {
	t.Helper()

	requestURL, err := url.Parse("http://" + addr + "/")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	srn, err := NewSarin(
		t.Context(),
		[]string{"GET"}, requestURL, 5*time.Second, workers, requests, duration,
		false, false,
		nil, types.Headers{{Key: "User-Agent", Value: []string{"sarin-test"}}}, nil, nil, nil, nil,
		true, false, "", "", nil, nil,
	)
	if err != nil {
		t.Fatalf("NewSarin: %v", err)
	}
	return srn
}

// TestRunSendsExactRequestCount is the end-to-end check that a counted run sends
// exactly the requested number of requests and records all of them.
func TestRunSendsExactRequestCount(t *testing.T) {
	addr, received := startTestServer(t)

	const want = 2000
	requests := uint64(want)
	srn := newTestSarin(t, addr, &requests, nil, 32)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	srn.Start(ctx, NewStopController(cancel))

	if got := received.Load(); got != want {
		t.Errorf("server received %d requests, want %d", got, want)
	}

	stats := srn.GetResponses().prepareOutputData()
	if got := stats.Total.Count.String(); got != "2000" {
		t.Errorf("recorded %s responses, want 2000", got)
	}
	if _, ok := stats.Responses["200"]; !ok {
		t.Errorf("no 200 responses recorded, got %v", stats.Responses)
	}
}

// TestRunStopsOnDuration checks that a duration-bounded run ends on its own and
// that cancelling stops it early.
func TestRunStopsOnDuration(t *testing.T) {
	addr, received := startTestServer(t)

	duration := 300 * time.Millisecond
	srn := newTestSarin(t, addr, nil, &duration, 8)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	start := time.Now()
	srn.Start(ctx, NewStopController(cancel))
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Errorf("run took %v, expected it to stop shortly after %v", elapsed, duration)
	}
	if received.Load() == 0 {
		t.Error("no requests were sent")
	}
}

// TestRunStopsOnCancel checks that cancelling the context ends the run.
func TestRunStopsOnCancel(t *testing.T) {
	addr, _ := startTestServer(t)

	srn := newTestSarin(t, addr, nil, nil, 8)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	time.AfterFunc(200*time.Millisecond, cancel)

	done := make(chan struct{})
	go func() {
		defer close(done)
		srn.Start(ctx, NewStopController(cancel))
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("run did not stop after the context was cancelled")
	}
}
