package cli

import "testing"

func TestIsGracefulStopTimeout(t *testing.T) {
	t.Parallel()

	if !isGracefulStopTimeout(assertErr("Failed shutting down instance, status is \"Running\": context deadline exceeded")) {
		t.Fatalf("expected graceful stop timeout detection to be true")
	}
	if isGracefulStopTimeout(assertErr("permission denied")) {
		t.Fatalf("expected non-timeout error to be false")
	}
}

type staticErr string

func (e staticErr) Error() string { return string(e) }

func assertErr(msg string) error { return staticErr(msg) }
