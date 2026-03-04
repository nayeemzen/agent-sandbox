package cli

import "testing"

func TestParsePortSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want portSpec
	}{
		{in: "8080:80", want: portSpec{HostPort: 8080, GuestPort: 80}},
		{in: "8000", want: portSpec{HostPort: 8000, GuestPort: 8000}},
		{in: ":8000", want: portSpec{GuestPort: 8000, RandomHostPort: true}},
		{in: "0:8000", want: portSpec{GuestPort: 8000, RandomHostPort: true}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()

			got, err := parsePortSpec(tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
		})
	}
}
