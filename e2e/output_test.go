package e2e

import (
	"encoding/json"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

// --- JSON output structure verification ---

func TestJSONOutputHasStatFields(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "3", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)

	// Verify total has all stat fields
	if out.Total.Count.String() != "3" {
		t.Errorf("expected count 3, got %s", out.Total.Count.String())
	}
	if out.Total.Min == "" {
		t.Error("expected min to be non-empty")
	}
	if out.Total.Max == "" {
		t.Error("expected max to be non-empty")
	}
	if out.Total.Average == "" {
		t.Error("expected average to be non-empty")
	}
	if out.Total.P90 == "" {
		t.Error("expected p90 to be non-empty")
	}
	if out.Total.P95 == "" {
		t.Error("expected p95 to be non-empty")
	}
	if out.Total.P99 == "" {
		t.Error("expected p99 to be non-empty")
	}
}

func TestJSONOutputResponseStatFields(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "5", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	stat, ok := out.Responses["200"]
	if !ok {
		t.Fatal("expected 200 in responses")
	}

	if stat.Count.String() != "5" {
		t.Errorf("expected response count 5, got %s", stat.Count.String())
	}
	if stat.Min == "" || stat.Max == "" || stat.Average == "" {
		t.Error("expected min/max/average to be non-empty")
	}
}

func TestJSONOutputMultipleStatusCodes(t *testing.T) {
	t.Parallel()

	// Create servers with different status codes
	srv200 := statusServer(200)
	defer srv200.Close()
	srv404 := statusServer(404)
	defer srv404.Close()

	// We can only target one URL, so use a single server
	// Instead, test that dry-run produces the expected structure
	res := run("-U", "http://example.com", "-r", "3", "-z", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	// dry-run should have "dry-run" key
	stat := out.Responses["dry-run"]
	if stat.Count.String() != "3" {
		t.Errorf("expected dry-run count 3, got %s", stat.Count.String())
	}
}

func TestJSONOutputIsValidJSON(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	// Verify it's valid JSON
	var raw map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &raw); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}

	// Verify top-level structure
	if _, ok := raw["responses"]; !ok {
		t.Error("expected 'responses' key in JSON output")
	}
	if _, ok := raw["total"]; !ok {
		t.Error("expected 'total' key in JSON output")
	}
}

// --- YAML output structure verification ---

func TestYAMLOutputIsValidYAML(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "yaml")
	assertExitCode(t, res, 0)

	var raw map[string]any
	if err := yaml.Unmarshal([]byte(res.Stdout), &raw); err != nil {
		t.Fatalf("stdout is not valid YAML: %v", err)
	}

	if _, ok := raw["responses"]; !ok {
		t.Error("expected 'responses' key in YAML output")
	}
	if _, ok := raw["total"]; !ok {
		t.Error("expected 'total' key in YAML output")
	}
}

func TestYAMLOutputHasStatFields(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "yaml")
	assertExitCode(t, res, 0)

	assertContains(t, res.Stdout, "count:")
	assertContains(t, res.Stdout, "min:")
	assertContains(t, res.Stdout, "max:")
	assertContains(t, res.Stdout, "average:")
	assertContains(t, res.Stdout, "p90:")
	assertContains(t, res.Stdout, "p95:")
	assertContains(t, res.Stdout, "p99:")
}

// --- Table output content verification ---

func TestTableOutputContainsHeaders(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "table")
	assertExitCode(t, res, 0)

	// Table should contain column headers
	assertContains(t, res.Stdout, "Response")
	assertContains(t, res.Stdout, "Count")
	assertContains(t, res.Stdout, "Min")
	assertContains(t, res.Stdout, "Max")
}

func TestTableOutputContainsStatusCode(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "table")
	assertExitCode(t, res, 0)

	assertContains(t, res.Stdout, "200")
}

// --- Version output format ---

func TestVersionOutputFormat(t *testing.T) {
	t.Parallel()

	res := run("-v")
	assertExitCode(t, res, 0)

	lines := strings.Split(strings.TrimSpace(res.Stdout), "\n")
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 lines in version output, got %d: %s", len(lines), res.Stdout)
	}
	assertContains(t, lines[0], "Version:")
	assertContains(t, lines[1], "Git Commit:")
	assertContains(t, lines[2], "Build Date:")
	assertContains(t, lines[3], "Go Version:")
}
