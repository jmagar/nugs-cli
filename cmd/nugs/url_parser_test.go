package main

import "testing"

func TestParsePaidLstreamShowID(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		want      string
		wantError bool
	}{
		{name: "valid", query: "showID=12345&foo=bar", want: "12345"},
		{name: "missing showID", query: "foo=bar", wantError: true},
		{name: "blank showID", query: "showID=   ", wantError: true},
		{name: "invalid query", query: "%", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePaidLstreamShowID(tt.query)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil and value %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected showID: got %q want %q", got, tt.want)
			}
		})
	}
}

func TestIsLikelyLivestreamSegments(t *testing.T) {
	tests := []struct {
		name      string
		segments  []string
		want      bool
		wantError bool
	}{
		{name: "no segments", segments: nil, wantError: true},
		{name: "single segment", segments: []string{"seg0.ts"}, want: false},
		{name: "same first two", segments: []string{"seg0.ts", "seg0.ts"}, want: false},
		{name: "different first two", segments: []string{"seg0.ts", "seg1.ts"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isLikelyLivestreamSegments(tt.segments)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected value: got %v want %v", got, tt.want)
			}
		})
	}
}
