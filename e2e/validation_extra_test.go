package e2e

import (
	"testing"
)

func TestValidation_ConcurrencyExceedsMax(t *testing.T) {
	t.Parallel()

	res := run("-U", "http://example.com", "-r", "1", "-q", "-c", "200000000")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "concurrency must not exceed 100,000,000")
}
