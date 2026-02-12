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
			name:         "lowercase transferred token",
			line:         "transferred: 1.0 GiB / 2.0 GiB, 50%, 20 MiB/s, ETA 10s",
			wantPercent:  50,
			wantSpeed:    "20 MiB/s",
			wantUploaded: "1.0 GiB",
			wantTotal:    "2.0 GiB",
			wantOK:       true,
		},
		{
			name:         "speed field before percent",
			line:         "NOTICE: Transferred: 52 MiB / 100 MiB, 1 MiB/s, 52%, ETA 1m",
			wantPercent:  52,
			wantSpeed:    "1 MiB/s",
			wantUploaded: "52 MiB",
			wantTotal:    "100 MiB",
			wantOK:       true,
		},
		{
			name:         "speed with at prefix",
			line:         "Transferred: 12 MiB / 24 MiB, 50%, @ 2 MiB/s, ETA 4s",
			wantPercent:  50,
			wantSpeed:    "2 MiB/s",
			wantUploaded: "12 MiB",
			wantTotal:    "24 MiB",
			wantOK:       true,
		},
		{
			name:         "missing percent uses computed fallback",
			line:         "Transferred: 256 MiB / 1 GiB, 16 MiB/s, ETA 48s",
			wantPercent:  25,
			wantSpeed:    "16 MiB/s",
			wantUploaded: "256 MiB",
			wantTotal:    "1 GiB",
			wantOK:       true,
		},
		{
			name:         "missing speed uses default",
			line:         "Transferred: 5 MiB / 10 MiB, 50%, ETA 5s",
			wantPercent:  50,
			wantSpeed:    "0 B",
			wantUploaded: "5 MiB",
			wantTotal:    "10 MiB",
			wantOK:       true,
		},
		{
			name:         "uploaded equals total fallback to full completion",
			line:         "Transferred: done / done, ETA 0s",
			wantPercent:  100,
			wantSpeed:    "0 B",
			wantUploaded: "done",
			wantTotal:    "done",
			wantOK:       true,
		},
		{
			name:         "ansi formatting stripped before parsing",
			line:         "\x1b[32mTransferred:\x1b[0m 80 MiB / 100 MiB, 80%, 8 MiB/s, ETA 3s",
			wantPercent:  80,
			wantSpeed:    "8 MiB/s",
			wantUploaded: "80 MiB",
			wantTotal:    "100 MiB",
			wantOK:       true,
		},
		{
			name:         "whitespace-heavy formatting normalized",
			line:         "Transferred:     80   MiB   /    100   MiB   ,  80 % ,  8 MiB/s  , ETA 3s",
			wantPercent:  80,
			wantSpeed:    "8 MiB/s",
			wantUploaded: "80 MiB",
			wantTotal:    "100 MiB",
			wantOK:       true,
		},
		{
			name:   "transferred present but missing slash",
			line:   "Transferred: 80 MiB, 80%, 8 MiB/s, ETA 3s",
			wantOK: false,
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
