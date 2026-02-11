package rclone

import "testing"

func TestParseRcloneProgressLine(t *testing.T) {
	tests := []struct {
		name         string
		line         string
		wantPercent  int
		wantSpeed    string
		wantUploaded string
		wantTotal    string
		wantOK       bool
	}{
		{
			name:         "standard one-line stats",
			line:         "Transferred:    52.403 MiB / 958.826 MiB, 5%, 10.284 MiB/s, ETA 1m31s",
			wantPercent:  5,
			wantSpeed:    "10.284 MiB/s",
			wantUploaded: "52.403 MiB",
			wantTotal:    "958.826 MiB",
			wantOK:       true,
		},
		{
			name:         "prefixed notice line",
			line:         "2026/02/08 10:00:00 NOTICE: Transferred: 959 MB / 959 MB, 100%, 167 MB/s, ETA 0s",
			wantPercent:  100,
			wantSpeed:    "167 MB/s",
			wantUploaded: "959 MB",
			wantTotal:    "959 MB",
			wantOK:       true,
		},
		{
			name:         "compact formatting without spaces",
			line:         "Transferred: 52.403MiB/958.826MiB,5%,10.284MiB/s,ETA 1m31s",
			wantPercent:  5,
			wantSpeed:    "10.284MiB/s",
			wantUploaded: "52.403MiB",
			wantTotal:    "958.826MiB",
			wantOK:       true,
		},
		{
			name:   "non-progress line",
			line:   "Checks: 0 / 0, -, Listed 1",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPercent, gotSpeed, gotUploaded, gotTotal, gotOK := ParseRcloneProgressLine(tt.line)

			if gotOK != tt.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}

			if gotPercent != tt.wantPercent {
				t.Errorf("percent = %d, want %d", gotPercent, tt.wantPercent)
			}
			if gotSpeed != tt.wantSpeed {
				t.Errorf("speed = %q, want %q", gotSpeed, tt.wantSpeed)
			}
			if gotUploaded != tt.wantUploaded {
				t.Errorf("uploaded = %q, want %q", gotUploaded, tt.wantUploaded)
			}
			if gotTotal != tt.wantTotal {
				t.Errorf("total = %q, want %q", gotTotal, tt.wantTotal)
			}
		})
	}
}
