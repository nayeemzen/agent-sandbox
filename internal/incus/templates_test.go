package incus

import (
	"strings"
	"testing"
)

func TestParseInstanceSource(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		in         string
		wantType   string
		wantAlias  string
		wantSource string
		wantErr    string
	}{
		{
			name:      "images alias",
			in:        "images:ubuntu/24.04",
			wantType:  "image",
			wantAlias: "ubuntu/24.04",
		},
		{
			name:      "local alias",
			in:        "local:my-image",
			wantType:  "image",
			wantAlias: "my-image",
		},
		{
			name:       "sandbox source",
			in:         "sandbox:base",
			wantType:   "copy",
			wantSource: "base",
		},
		{
			name:    "empty",
			in:      "",
			wantErr: "invalid source",
		},
		{
			name:    "invalid images",
			in:      "images:",
			wantErr: "invalid source",
		},
		{
			name:    "invalid sandbox",
			in:      "sandbox:",
			wantErr: "invalid source",
		},
		{
			name:    "unsupported",
			in:      "docker://alpine",
			wantErr: "unsupported source",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseInstanceSource(tc.in)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.wantErr)) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Type != tc.wantType {
				t.Fatalf("type=%q, want %q", got.Type, tc.wantType)
			}
			if got.Alias != tc.wantAlias {
				t.Fatalf("alias=%q, want %q", got.Alias, tc.wantAlias)
			}
			if got.Source != tc.wantSource {
				t.Fatalf("source=%q, want %q", got.Source, tc.wantSource)
			}
		})
	}
}
