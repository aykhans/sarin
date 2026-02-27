package e2e

import (
	"encoding/json"
	"syscall"
	"testing"
	"time"
)

func TestSIGINTGracefulShutdown(t *testing.T) {
	t.Parallel()
	srv := slowServer(100 * time.Millisecond)
	defer srv.Close()

	// Start a duration-based test that would run for a long time
	cmd, stdout := startProcess(
		"-U", srv.URL, "-d", "30s", "-q", "-o", "json",
	)

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	// Let it run for a bit so some requests complete
	time.Sleep(500 * time.Millisecond)

	// Send SIGINT for graceful shutdown
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("failed to send SIGINT: %v", err)
	}

	// Wait for process to exit
	err := cmd.Wait()
	_ = err // May exit with 0 or non-zero depending on timing

	// Should have produced valid JSON output with partial results
	output := stdout.String()
	if output == "" {
		t.Fatal("expected JSON output after SIGINT, got empty stdout")
	}

	var out outputData
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatalf("expected valid JSON after graceful shutdown: %v\nstdout: %s", err, output)
	}

	count, _ := out.Total.Count.Int64()
	if count < 1 {
		t.Errorf("expected at least 1 request before shutdown, got %d", count)
	}
}

func TestSIGTERMGracefulShutdown(t *testing.T) {
	t.Parallel()
	srv := slowServer(100 * time.Millisecond)
	defer srv.Close()

	cmd, stdout := startProcess(
		"-U", srv.URL, "-d", "30s", "-q", "-o", "json",
	)

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM: %v", err)
	}

	err := cmd.Wait()
	_ = err

	output := stdout.String()
	if output == "" {
		t.Fatal("expected JSON output after SIGTERM, got empty stdout")
	}

	var out outputData
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatalf("expected valid JSON after graceful shutdown: %v\nstdout: %s", err, output)
	}
}

func TestSIGINTExitsInReasonableTime(t *testing.T) {
	t.Parallel()
	srv := slowServer(50 * time.Millisecond)
	defer srv.Close()

	cmd, _ := startProcess(
		"-U", srv.URL, "-d", "60s", "-q", "-o", "none",
	)

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("failed to send SIGINT: %v", err)
	}

	// Should exit within 5 seconds
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-done:
		// Good — exited in time
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		t.Fatal("process did not exit within 5 seconds after SIGINT")
	}
}
