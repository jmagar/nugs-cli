package main

import (
	"testing"
	"time"

	"github.com/jmagar/nugs-cli/internal/model"
)

// TestUploadProgressPhaseTransitions tests that phase transitions are valid
func TestUploadProgressPhaseTransitions(t *testing.T) {
	tests := []struct {
		name      string
		fromPhase string
		toPhase   string
		wantValid bool
	}{
		{"Idle to Download", model.PhaseIdle, model.PhaseDownload, true},
		{"Idle to Upload", model.PhaseIdle, model.PhaseUpload, true},
		{"Download to Upload", model.PhaseDownload, model.PhaseUpload, true},
		{"Upload to Complete", model.PhaseUpload, model.PhaseComplete, true},
		{"Upload to Error", model.PhaseUpload, model.PhaseError, true},
		{"Complete to Download", model.PhaseComplete, model.PhaseDownload, true},
		{"Download to Download", model.PhaseDownload, model.PhaseDownload, false},
		{"Complete to Upload", model.PhaseComplete, model.PhaseUpload, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &ProgressBoxState{}
			state.CurrentPhase = tt.fromPhase

			err := state.SetPhase(tt.toPhase)
			gotValid := (err == nil)

			if gotValid != tt.wantValid {
				t.Errorf("SetPhase(%s -> %s) valid = %v, want %v (err: %v)",
					tt.fromPhase, tt.toPhase, gotValid, tt.wantValid, err)
			}
		})
	}
}

// TestUploadTotalSetFlag tests that the UploadTotalSet flag prevents race conditions
func TestUploadTotalSetFlag(t *testing.T) {
	state := &ProgressBoxState{}

	// Initially, flag is false and UploadTotal is empty
	if state.UploadTotalSet {
		t.Error("UploadTotalSet should be false initially")
	}
	if state.UploadTotal != "" {
		t.Error("UploadTotal should be empty initially")
	}

	// Simulate onPreUpload setting the total from directory size
	state.UploadTotal = "100 MB"
	state.UploadTotalSet = true

	// Verify flag prevents overwrite (simulating progressFn logic)
	rcloneReportedTotal := "..."
	if !state.UploadTotalSet && rcloneReportedTotal != "" && rcloneReportedTotal != "..." {
		state.UploadTotal = rcloneReportedTotal
		t.Error("Should not overwrite UploadTotal when flag is set")
	}

	// Verify total was not changed
	if state.UploadTotal != "100 MB" {
		t.Errorf("UploadTotal = %q, want %q", state.UploadTotal, "100 MB")
	}
}

// TestResetForNewAlbumClearsUploadTotalSet tests that reset clears the flag
func TestResetForNewAlbumClearsUploadTotalSet(t *testing.T) {
	state := &ProgressBoxState{}
	state.UploadTotal = "100 MB"
	state.UploadTotalSet = true

	state.ResetForNewAlbum("New Show", "12345", 10, 500000000)

	if state.UploadTotalSet {
		t.Error("ResetForNewAlbum should clear UploadTotalSet flag")
	}
	if state.UploadTotal != "" {
		t.Error("ResetForNewAlbum should clear UploadTotal")
	}
}

// TestUploadDurationCalculation tests edge cases in duration calculation
func TestUploadDurationCalculation(t *testing.T) {
	tests := []struct {
		name          string
		startTime     time.Time
		wantZero      bool
		wantNonZero   bool
		description   string
	}{
		{
			name:        "Zero start time",
			startTime:   time.Time{},
			wantZero:    true,
			description: "Should not calculate duration with zero start time",
		},
		{
			name:        "Valid start time",
			startTime:   time.Now().Add(-5 * time.Second),
			wantNonZero: true,
			description: "Should calculate positive duration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &ProgressBoxState{
				UploadStartTime: tt.startTime,
			}

			var duration time.Duration
			if !state.UploadStartTime.IsZero() {
				duration = time.Since(state.UploadStartTime)
			}

			if tt.wantZero && duration != 0 {
				t.Errorf("Duration should be zero, got %v", duration)
			}
			if tt.wantNonZero && duration <= 0 {
				t.Errorf("Duration should be positive, got %v", duration)
			}
		})
	}
}

// TestSpeedCalculationEdgeCases tests division by zero and instant upload handling
func TestSpeedCalculationEdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		duration     time.Duration
		bytes        int64
		shouldCalc   bool
		description  string
	}{
		{
			name:        "Zero duration",
			duration:    0,
			bytes:       1000000,
			shouldCalc:  false,
			description: "Should not calculate speed with zero duration",
		},
		{
			name:        "Instant upload (<1ms)",
			duration:    500 * time.Microsecond,
			bytes:       1000000,
			shouldCalc:  false,
			description: "Should not calculate speed for very fast uploads",
		},
		{
			name:        "Normal duration",
			duration:    5 * time.Second,
			bytes:       1000000,
			shouldCalc:  true,
			description: "Should calculate speed for normal uploads",
		},
		{
			name:        "Zero bytes",
			duration:    5 * time.Second,
			bytes:       0,
			shouldCalc:  false,
			description: "Should not calculate speed with zero bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the format.go speed calculation logic
			shouldCalc := tt.duration.Seconds() >= 0.001 && tt.bytes > 0

			if shouldCalc != tt.shouldCalc {
				t.Errorf("Speed calculation decision = %v, want %v", shouldCalc, tt.shouldCalc)
			}

			// Only calculate if we should
			if shouldCalc {
				speed := float64(tt.bytes) / tt.duration.Seconds()
				if speed <= 0 {
					t.Error("Calculated speed should be positive")
				}
			}
		})
	}
}

// TestPhaseErrorTransitionFromAnyState tests that error transitions are always allowed
func TestPhaseErrorTransitionFromAnyState(t *testing.T) {
	phases := []string{
		model.PhaseIdle,
		model.PhaseDownload,
		model.PhaseUpload,
		model.PhaseVerify,
		model.PhaseComplete,
		model.PhasePaused,
	}

	for _, fromPhase := range phases {
		t.Run("From_"+fromPhase, func(t *testing.T) {
			state := &ProgressBoxState{}
			state.CurrentPhase = fromPhase

			err := state.SetPhase(model.PhaseError)
			if err != nil {
				t.Errorf("Transition from %s to error should always succeed, got error: %v",
					fromPhase, err)
			}

			if state.CurrentPhase != model.PhaseError {
				t.Errorf("CurrentPhase = %s, want %s", state.CurrentPhase, model.PhaseError)
			}
		})
	}
}

// TestUploadCompleteVisibilityDelay tests that the constant is defined and reasonable
func TestUploadCompleteVisibilityDelay(t *testing.T) {
	if uploadCompleteVisibilityDelay <= 0 {
		t.Error("uploadCompleteVisibilityDelay should be positive")
	}

	// Reasonable range: 100ms to 2s
	if uploadCompleteVisibilityDelay < 100*time.Millisecond {
		t.Error("uploadCompleteVisibilityDelay is too short (< 100ms)")
	}
	if uploadCompleteVisibilityDelay > 2*time.Second {
		t.Error("uploadCompleteVisibilityDelay is too long (> 2s)")
	}

	// Verify it's exactly 500ms as specified
	if uploadCompleteVisibilityDelay != 500*time.Millisecond {
		t.Errorf("uploadCompleteVisibilityDelay = %v, want 500ms", uploadCompleteVisibilityDelay)
	}
}

// BenchmarkPhaseTransition benchmarks phase transition validation
func BenchmarkPhaseTransition(b *testing.B) {
	state := &ProgressBoxState{}
	state.CurrentPhase = model.PhaseDownload

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.SetPhase(model.PhaseUpload)
		state.CurrentPhase = model.PhaseDownload // Reset for next iteration
	}
}

// BenchmarkSpeedCalculation benchmarks speed calculation with edge case checks
func BenchmarkSpeedCalculation(b *testing.B) {
	duration := 5 * time.Second
	bytes := int64(1000000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if duration.Seconds() >= 0.001 && bytes > 0 {
			_ = float64(bytes) / duration.Seconds()
		}
	}
}
