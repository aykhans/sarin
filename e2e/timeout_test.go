package e2e

import (
	"testing"
	"time"
)

func TestRequestTimeout(t *testing.T) {
	t.Parallel()

	// Server that takes 2 seconds to respond
	srv := slowServer(2 * time.Second)
	defer srv.Close()

	// Timeout of 200ms — should fail with timeout error
	res := run("-U", srv.URL, "-r", "1", "-T", "200ms", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	// Should NOT have "200" — should have a timeout error
	if _, ok := out.Responses["200"]; ok {
		t.Error("expected timeout error, but got 200")
	}
	// Total count should still be 1 (the timed-out request is counted)
	assertResponseCount(t, out, 1)
}

func TestRequestTimeoutMultiple(t *testing.T) {
	t.Parallel()

	srv := slowServer(2 * time.Second)
	defer srv.Close()

	res := run("-U", srv.URL, "-r", "3", "-c", "3", "-T", "200ms", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertResponseCount(t, out, 3)

	// None should be 200
	if _, ok := out.Responses["200"]; ok {
		t.Error("expected all requests to timeout, but got some 200s")
	}
}

func TestTimeoutDoesNotAffectFastRequests(t *testing.T) {
	t.Parallel()
	srv := echoServer()
	defer srv.Close()

	// Short timeout but server responds instantly — should succeed
	res := run("-U", srv.URL, "-r", "3", "-T", "5s", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertHasResponseKey(t, out, "200")
	assertResponseCount(t, out, 3)
}

func TestDurationStopsAfterTime(t *testing.T) {
	t.Parallel()
	srv := echoServer()
	defer srv.Close()

	start := time.Now()
	res := run("-U", srv.URL, "-d", "1s", "-q", "-o", "json")
	elapsed := time.Since(start)

	assertExitCode(t, res, 0)

	// Should finish roughly around 1s (allow some tolerance)
	if elapsed < 900*time.Millisecond {
		t.Errorf("expected test to run ~1s, but finished in %v", elapsed)
	}
	if elapsed > 3*time.Second {
		t.Errorf("expected test to finish around 1s, but took %v", elapsed)
	}
}

func TestDurationWithRequestLimit(t *testing.T) {
	t.Parallel()
	srv := echoServer()
	defer srv.Close()

	// Request limit reached before duration — should stop early
	res := run("-U", srv.URL, "-r", "2", "-d", "30s", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertResponseCount(t, out, 2)
}

func TestDurationWithSlowServerStopsAtDuration(t *testing.T) {
	t.Parallel()

	// Server delays 500ms per request
	srv := slowServer(500 * time.Millisecond)
	defer srv.Close()

	start := time.Now()
	res := run("-U", srv.URL, "-d", "1s", "-c", "1", "-q", "-o", "json")
	elapsed := time.Since(start)

	assertExitCode(t, res, 0)

	// Should stop after ~1s even though requests are slow
	if elapsed > 3*time.Second {
		t.Errorf("expected to stop around 1s duration, took %v", elapsed)
	}
}
