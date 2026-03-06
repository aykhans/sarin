package e2e

import (
	"testing"
)

func TestStringToUpper(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-H", `X-Upper: {{ strings_ToUpper "hello" }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-Upper"]; len(v) == 0 || v[0] != "HELLO" {
		t.Errorf("expected X-Upper: HELLO, got %v", v)
	}
}

func TestStringToLower(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-H", `X-Lower: {{ strings_ToLower "WORLD" }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-Lower"]; len(v) == 0 || v[0] != "world" {
		t.Errorf("expected X-Lower: world, got %v", v)
	}
}

func TestStringReplace(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-B", `{{ strings_Replace "foo-bar-baz" "-" "_" -1 }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Body != "foo_bar_baz" {
		t.Errorf("expected body foo_bar_baz, got %q", req.Body)
	}
}

func TestStringRemoveSpaces(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-B", `{{ strings_RemoveSpaces "hello world foo" }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Body != "helloworldfoo" {
		t.Errorf("expected body helloworldfoo, got %q", req.Body)
	}
}

func TestStringTrimPrefix(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-B", `{{ strings_TrimPrefix "hello-world" "hello-" }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Body != "world" {
		t.Errorf("expected body world, got %q", req.Body)
	}
}

func TestStringTrimSuffix(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-B", `{{ strings_TrimSuffix "hello-world" "-world" }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Body != "hello" {
		t.Errorf("expected body hello, got %q", req.Body)
	}
}

func TestSliceJoin(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-B", `{{ slice_Join (slice_Str "a" "b" "c") ", " }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Body != "a, b, c" {
		t.Errorf("expected body 'a, b, c', got %q", req.Body)
	}
}

func TestStringFirst(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-B", `{{ strings_First "abcdef" 3 }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Body != "abc" {
		t.Errorf("expected body abc, got %q", req.Body)
	}
}

func TestStringLast(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-B", `{{ strings_Last "abcdef" 3 }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Body != "def" {
		t.Errorf("expected body def, got %q", req.Body)
	}
}

func TestStringTruncate(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-B", `{{ strings_Truncate "hello world" 5 }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Body != "hello..." {
		t.Errorf("expected body 'hello...', got %q", req.Body)
	}
}

func TestSliceStr(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-B", `{{ slice_Join (slice_Str "a" "b" "c") "-" }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Body != "a-b-c" {
		t.Errorf("expected body a-b-c, got %q", req.Body)
	}
}
