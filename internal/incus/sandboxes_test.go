package incus

import "testing"

func TestShouldUseSIGTERMHaltForSource(t *testing.T) {
	t.Parallel()

	cases := []struct {
		source string
		want   bool
	}{
		{source: "images:alpine/3.20", want: true},
		{source: "local:alpine/edge", want: true},
		{source: "images:ubuntu/24.04", want: false},
		{source: "images:debian/12", want: false},
		{source: "", want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.source, func(t *testing.T) {
			t.Parallel()
			if got := shouldUseSIGTERMHaltForSource(tc.source); got != tc.want {
				t.Fatalf("shouldUseSIGTERMHaltForSource(%q)=%v, want %v", tc.source, got, tc.want)
			}
		})
	}
}

func TestRawLXCContainsHaltSignal(t *testing.T) {
	t.Parallel()

	if rawLXCContainsHaltSignal("") {
		t.Fatalf("empty raw.lxc should not contain halt signal")
	}
	if !rawLXCContainsHaltSignal("lxc.signal.halt = SIGTERM") {
		t.Fatalf("expected halt signal detection")
	}
	if !rawLXCContainsHaltSignal("lxc.apparmor.profile = unconfined\nlxc.signal.halt=SIGPWR") {
		t.Fatalf("expected halt signal detection in multi-line config")
	}
}

func TestAppendRawLXCLine(t *testing.T) {
	t.Parallel()

	if got := appendRawLXCLine("", "lxc.signal.halt = SIGTERM"); got != "lxc.signal.halt = SIGTERM" {
		t.Fatalf("got %q", got)
	}

	got := appendRawLXCLine("lxc.apparmor.profile = unconfined", "lxc.signal.halt = SIGTERM")
	want := "lxc.apparmor.profile = unconfined\nlxc.signal.halt = SIGTERM"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
