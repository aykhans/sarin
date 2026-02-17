package e2e

import (
	"encoding/base64"
	"testing"
)

func TestBodyFormDataSimple(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-B", `{{ body_FormData "name" "John" "age" "30" }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	// Body should contain multipart form data
	assertContains(t, req.Body, "name")
	assertContains(t, req.Body, "John")
	assertContains(t, req.Body, "age")
	assertContains(t, req.Body, "30")

	// Content-Type should be multipart/form-data
	ct := req.Headers["Content-Type"]
	if len(ct) == 0 {
		t.Fatal("expected Content-Type header for form data")
	}
	assertContains(t, ct[0], "multipart/form-data")
}

func TestBodyFormDataWithFileUpload(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// Create a temp file to upload
	filePath := writeTemp(t, "upload.txt", "file content here")

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-B", `{{ body_FormData "description" "test file" "document" "@`+filePath+`" }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	assertContains(t, req.Body, "description")
	assertContains(t, req.Body, "test file")
	assertContains(t, req.Body, "file content here")
	assertContains(t, req.Body, "upload.txt")
}

func TestBodyFormDataWithRemoteFile(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// Serve a file via HTTP
	fileServer := statusServerWithBody("remote file content")
	defer fileServer.Close()

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-B", `{{ body_FormData "file" "@`+fileServer.URL+`" }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	assertContains(t, req.Body, "remote file content")
}

func TestBodyFormDataEscapedAt(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// @@ should send literal @ prefixed value
	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-B", `{{ body_FormData "email" "@@user@example.com" }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	assertContains(t, req.Body, "@user@example.com")
}

func TestBodyFormDataOddArgsError(t *testing.T) {
	t.Parallel()

	// Odd number of args should cause an error
	res := run("-U", "http://example.com", "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-B", `{{ body_FormData "key_only" }}`)
	// This should either fail at validation or produce an error in output
	// The template is valid syntax but body_FormData returns an error at runtime
	if res.ExitCode == 0 {
		out := res.jsonOutput(t)
		// If it didn't exit 1, the error should show up as a response key
		if _, ok := out.Responses["200"]; ok {
			t.Error("expected error for odd form data args, but got 200")
		}
	}
}

func TestFileBase64(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	content := "hello base64 world"
	filePath := writeTemp(t, "base64test.txt", content)
	expected := base64.StdEncoding.EncodeToString([]byte(content))

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-B", `{{ file_Base64 "`+filePath+`" }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Body != expected {
		t.Errorf("expected base64 %q, got %q", expected, req.Body)
	}
}

func TestFileBase64RemoteFile(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	content := "remote base64 content"
	fileServer := statusServerWithBody(content)
	defer fileServer.Close()

	expected := base64.StdEncoding.EncodeToString([]byte(content))

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-B", `{{ file_Base64 "`+fileServer.URL+`" }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Body != expected {
		t.Errorf("expected base64 %q, got %q", expected, req.Body)
	}
}

func TestBodyFormDataMultipleRequests(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "3", "-c", "1", "-M", "POST", "-q", "-o", "json",
		"-B", `{{ body_FormData "id" "{{ fakeit_UUID }}" }}`)
	assertExitCode(t, res, 0)

	assertResponseCount(t, res.jsonOutput(t), 3)
}
