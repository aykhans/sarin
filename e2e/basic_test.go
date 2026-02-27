package e2e

import (
	"strconv"
	"testing"
)

func TestNoArgs(t *testing.T) {
	t.Parallel()
	res := run()
	assertExitCode(t, res, 1)
	// With no args and no env vars, validation should fail on required fields
	assertContains(t, res.Stderr, "VALIDATION")
}

func TestHelp(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"-h", "-help"} {
		t.Run(flag, func(t *testing.T) {
			t.Parallel()
			res := run(flag)
			assertContains(t, res.Stdout, "Usage:")
			assertContains(t, res.Stdout, "-url")
		})
	}
}

func TestVersion(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"-v", "-version"} {
		t.Run(flag, func(t *testing.T) {
			t.Parallel()
			res := run(flag)
			assertExitCode(t, res, 0)
			assertContains(t, res.Stdout, "Version:")
			assertContains(t, res.Stdout, "Git Commit:")
		})
	}
}

func TestUnexpectedArgs(t *testing.T) {
	t.Parallel()
	res := run("-U", "http://example.com", "unexpected")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "Unexpected CLI arguments")
}

func TestSimpleRequest(t *testing.T) {
	t.Parallel()
	srv := echoServer()
	defer srv.Close()

	res := run("-U", srv.URL, "-r", "3", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertHasResponseKey(t, out, "200")
	assertResponseCount(t, out, 3)
}

func TestDryRun(t *testing.T) {
	t.Parallel()
	res := run("-U", "http://example.com", "-r", "5", "-q", "-o", "json", "-z")
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertHasResponseKey(t, out, "dry-run")
	assertResponseCount(t, out, 5)
}

func TestDryRunDoesNotSendRequests(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "5", "-q", "-o", "json", "-z")
	assertExitCode(t, res, 0)

	if cs.requestCount() != 0 {
		t.Errorf("dry-run should not send any requests, but server received %d", cs.requestCount())
	}
}

func TestQuietMode(t *testing.T) {
	t.Parallel()
	srv := echoServer()
	defer srv.Close()

	res := run("-U", srv.URL, "-r", "1", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	if res.Stderr != "" {
		t.Errorf("expected empty stderr in quiet mode, got: %s", res.Stderr)
	}
}

func TestOutputNone(t *testing.T) {
	t.Parallel()
	srv := echoServer()
	defer srv.Close()

	res := run("-U", srv.URL, "-r", "1", "-q", "-o", "none")
	assertExitCode(t, res, 0)

	if res.Stdout != "" {
		t.Errorf("expected empty stdout with -o none, got: %s", res.Stdout)
	}
}

func TestOutputJSON(t *testing.T) {
	t.Parallel()
	srv := echoServer()
	defer srv.Close()

	res := run("-U", srv.URL, "-r", "1", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	if out.Responses == nil {
		t.Fatal("responses field is nil")
	}
	if out.Total.Min == "" || out.Total.Max == "" || out.Total.Average == "" {
		t.Errorf("total stats are incomplete: %+v", out.Total)
	}
	if out.Total.P90 == "" || out.Total.P95 == "" || out.Total.P99 == "" {
		t.Errorf("total percentiles are incomplete: %+v", out.Total)
	}
}

func TestOutputYAML(t *testing.T) {
	t.Parallel()
	srv := echoServer()
	defer srv.Close()

	res := run("-U", srv.URL, "-r", "1", "-q", "-o", "yaml")
	assertExitCode(t, res, 0)

	assertContains(t, res.Stdout, "responses:")
	assertContains(t, res.Stdout, "total:")
	assertContains(t, res.Stdout, "count:")
}

func TestOutputTable(t *testing.T) {
	t.Parallel()
	srv := echoServer()
	defer srv.Close()

	res := run("-U", srv.URL, "-r", "1", "-q", "-o", "table")
	assertExitCode(t, res, 0)

	assertContains(t, res.Stdout, "Response")
	assertContains(t, res.Stdout, "Count")
	assertContains(t, res.Stdout, "Min")
	assertContains(t, res.Stdout, "P99")
}

func TestInvalidOutputFormat(t *testing.T) {
	t.Parallel()
	res := run("-U", "http://example.com", "-r", "1", "-o", "invalid")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "Output")
}

func TestStatusCodes(t *testing.T) {
	t.Parallel()
	codes := []int{200, 201, 204, 301, 400, 404, 500, 502}
	for _, code := range codes {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			t.Parallel()
			srv := statusServer(code)
			defer srv.Close()

			res := run("-U", srv.URL, "-r", "1", "-q", "-o", "json")
			assertExitCode(t, res, 0)

			out := res.jsonOutput(t)
			assertHasResponseKey(t, out, strconv.Itoa(code))
		})
	}
}

func TestConcurrency(t *testing.T) {
	t.Parallel()
	srv := echoServer()
	defer srv.Close()

	res := run("-U", srv.URL, "-r", "10", "-c", "5", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertResponseCount(t, out, 10)
}

func TestDuration(t *testing.T) {
	t.Parallel()
	srv := echoServer()
	defer srv.Close()

	res := run("-U", srv.URL, "-d", "1s", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	count, _ := out.Total.Count.Int64()
	if count < 1 {
		t.Errorf("expected at least 1 request during 1s duration, got %d", count)
	}
}

func TestRequestsAndDuration(t *testing.T) {
	t.Parallel()
	srv := echoServer()
	defer srv.Close()

	// Both -r and -d set: should stop at whichever comes first
	res := run("-U", srv.URL, "-r", "3", "-d", "10s", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertResponseCount(t, out, 3)
}
