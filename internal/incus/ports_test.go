package incus

import "testing"

func TestParseProxyEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in        string
		wantProto string
		wantHost  string
		wantPort  int
		wantErr   bool
	}{
		{in: "tcp:0.0.0.0:8080", wantProto: "tcp", wantHost: "0.0.0.0", wantPort: 8080},
		{in: "udp:127.0.0.1:53", wantProto: "udp", wantHost: "127.0.0.1", wantPort: 53},
		{in: "tcp:[::]:1234", wantProto: "tcp", wantHost: "::", wantPort: 1234},
		{in: "", wantErr: true},
		{in: "tcp:", wantErr: true},
		{in: "tcp:0.0.0.0", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()

			proto, host, port, err := parseProxyEndpoint(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if proto != tc.wantProto {
				t.Fatalf("proto=%q want %q", proto, tc.wantProto)
			}
			if host != tc.wantHost {
				t.Fatalf("host=%q want %q", host, tc.wantHost)
			}
			if port != tc.wantPort {
				t.Fatalf("port=%d want %d", port, tc.wantPort)
			}
		})
	}
}

func TestPortDeviceName(t *testing.T) {
	t.Parallel()

	got := portDeviceName("tcp", 8080, 80)
	if got != "sandbox-port-tcp-8080-80" {
		t.Fatalf("got %q", got)
	}
}
