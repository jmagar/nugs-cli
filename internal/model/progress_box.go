package model

import (
	"fmt"
	"sync"
	"time"
)

// DefaultProgressRenderInterval is the minimum time between progress box redraws.
const DefaultProgressRenderInterval = 100 * time.Millisecond

// Phase constants for operation lifecycle tracking
const (
	PhaseIdle     = ""         // No operation in progress
	PhaseDownload = "download" // Downloading tracks
	PhaseUpload   = "upload"   // Uploading to remote storage
	PhaseVerify   = "verify"   // Verifying upload integrity
	PhaseComplete = "complete" // Operation completed successfully
	PhaseError    = "error"    // Operation failed with error
	PhasePaused   = "paused"   // Operation paused by user
)

// ProgressBoxState tracks the state of the dual progress box display.
type ProgressBoxState struct {
	Mu sync.Mutex // Protects all fields from concurrent access

	ShowTitle         string
	ShowNumber        string
	TrackNumber       int
	TrackTotal        int
	TrackName         string
	TrackFormat       string
	DownloadPercent   int
	DownloadSpeed     string
	Downloaded        string
	DownloadTotal     string
	UploadPercent     int
	UploadSpeed       string
	Uploaded          string
	UploadTotal       string
	UploadTotalSet    bool // Flag to prevent overwriting calculated upload total
	ShowPercent       int
	ShowDownloaded    string
	ShowTotal         string
	AccumulatedBytes  int64 // Accumulated download across all tracks
	AccumulatedTracks int   // Number of completed tracks
	RcloneEnabled     bool
	LinesDrawn        int

	// ETA tracking
	DownloadETA        string        // Estimated time remaining for download
	UploadETA          string        // Estimated time remaining for upload
	SpeedHistory       []float64     // Last 10 download speed samples for smoothing
	UploadSpeedHistory []float64     // Last 10 upload speed samples for smoothing (thread-safe)
	LastUpdateTime     time.Time     // Last time the progress box was rendered
	RenderInterval     time.Duration // Minimum interval between redraws (defaults to 100ms)

	// Completion tracking
	IsComplete     bool          // Whether all tracks have completed
	CompletionTime time.Time     // When the download completed
	TotalDuration  time.Duration // Total time taken for the show
	SkippedTracks  int           // Number of tracks skipped (already exist)
	ErrorTracks    int           // Number of tracks that failed
	StartTime      time.Time     // When the download started

	// Upload timing tracking
	UploadStartTime time.Time     // When the upload started
	UploadDuration  time.Duration // Total time taken for upload

	// Phase tracking (Tier 1 enhancement)
	CurrentPhase string // Current operation phase (download, upload, verify, paused, error)
	StatusColor  string // ANSI color code for current phase

	// Message display (Tier 3 enhancement)
	ErrorMessage    string    // Current error message to display
	WarningMessage  string    // Current warning message to display
	StatusMessage   string    // Current status message to display
	MessageExpiry   time.Time // When the current message should expire
	MessagePriority int       // Priority of current message (1=status, 2=warning, 3=error)

	// State indicators (Tier 3 enhancement)
	IsPaused    bool // Whether the download/upload is paused
	IsCancelled bool // Whether the operation was cancelled

	// Batch progress (Tier 4 enhancement)
	BatchState *BatchProgressState // Optional batch context for multi-album operations

	// Render-state tracking for smart redraw decisions
	LastRenderedTrackNumber     int
	LastRenderedMessagePriority int
	LastRenderedPaused          bool
	LastRenderedCancelled       bool
	ForceRender                 bool
}

// SetPhase sets the current operation phase with transition validation.
// Returns an error if the phase transition is invalid.
// Thread-safe: acquires mutex internally.
func (s *ProgressBoxState) SetPhase(phase string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	// Validate transition
	if !s.isValidTransition(s.CurrentPhase, phase) {
		return fmt.Errorf("invalid phase transition: %s -> %s", s.CurrentPhase, phase)
	}

	s.CurrentPhase = phase
	s.ForceRender = true
	return nil
}

// SetPhaseLocked sets the phase without validation (caller must hold Mu).
// Use this when you've already validated the transition or need to bypass validation.
func (s *ProgressBoxState) SetPhaseLocked(phase string) {
	s.CurrentPhase = phase
	s.ForceRender = true
}

// isValidTransition checks if a phase transition is allowed.
// REQUIRES: Caller must hold s.Mu lock.
func (s *ProgressBoxState) isValidTransition(from, to string) bool {
	// Allow transitions to error from any state
	if to == PhaseError {
		return true
	}

	// Allow transitions to complete from any active state
	if to == PhaseComplete && (from == PhaseDownload || from == PhaseUpload || from == PhaseVerify) {
		return true
	}

	// Valid state machine transitions
	switch from {
	case PhaseIdle:
		return to == PhaseDownload || to == PhaseUpload
	case PhaseDownload:
		return to == PhaseUpload || to == PhaseVerify || to == PhaseComplete || to == PhasePaused
	case PhaseUpload:
		return to == PhaseVerify || to == PhaseComplete || to == PhasePaused
	case PhaseVerify:
		return to == PhaseComplete || to == PhaseError
	case PhasePaused:
		return to == PhaseDownload || to == PhaseUpload || to == PhaseVerify
	case PhaseComplete, PhaseError:
		return to == PhaseIdle || to == PhaseDownload // Can restart after completion/error
	default:
		return true // Unknown phases allowed for backward compatibility
	}
}

// GetRenderIntervalLocked returns the render interval (caller must hold Mu).
func (s *ProgressBoxState) GetRenderIntervalLocked() time.Duration {
	if s.RenderInterval <= 0 {
		return DefaultProgressRenderInterval
	}
	return s.RenderInterval
}

// HasCriticalRenderChangeLocked checks if a critical state change occurred (caller must hold Mu).
func (s *ProgressBoxState) HasCriticalRenderChangeLocked() bool {
	return s.TrackNumber != s.LastRenderedTrackNumber ||
		s.MessagePriority != s.LastRenderedMessagePriority ||
		s.IsPaused != s.LastRenderedPaused ||
		s.IsCancelled != s.LastRenderedCancelled
}

// ShouldRenderLocked checks if a render is needed and updates tracking state (caller must hold Mu).
func (s *ProgressBoxState) ShouldRenderLocked(now time.Time) bool {
	force := s.ForceRender || s.HasCriticalRenderChangeLocked()
	if !force && !s.LastUpdateTime.IsZero() &&
		now.Sub(s.LastUpdateTime) < s.GetRenderIntervalLocked() {
		return false
	}

	s.LastUpdateTime = now
	s.ForceRender = false
	s.LastRenderedTrackNumber = s.TrackNumber
	s.LastRenderedMessagePriority = s.MessagePriority
	s.LastRenderedPaused = s.IsPaused
	s.LastRenderedCancelled = s.IsCancelled
	return true
}

// ShouldRender checks if a render is needed (thread-safe).
func (s *ProgressBoxState) ShouldRender(now time.Time) bool {
	if s == nil {
		return false
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()
	return s.ShouldRenderLocked(now)
}

// RequestRender marks the progress box to redraw on the next render attempt.
func (s *ProgressBoxState) RequestRender() {
	if s == nil {
		return
	}
	s.Mu.Lock()
	s.ForceRender = true
	s.Mu.Unlock()
}

// SetMessage sets a temporary message with priority and duration (Tier 3 enhancement).
// Priority: 1=Status (cyan), 2=Warning (yellow), 3=Error (red).
// Duration: how long the message should be displayed before expiring.
// Thread-safe: can be called concurrently with progress updates.
func (s *ProgressBoxState) SetMessage(priority int, text string, duration time.Duration) {
	if s == nil {
		return
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()

	// Check if current message has expired
	messageExpired := !s.MessageExpiry.IsZero() && time.Now().After(s.MessageExpiry)

	// Only update if new message has higher or equal priority, or current message expired
	if priority >= s.MessagePriority || messageExpired {
		s.MessagePriority = priority
		s.MessageExpiry = time.Now().Add(duration)

		// Clear all messages first
		s.StatusMessage = ""
		s.WarningMessage = ""
		s.ErrorMessage = ""

		// Set the appropriate message based on priority
		switch priority {
		case MessagePriorityError:
			s.ErrorMessage = text
		case MessagePriorityWarning:
			s.WarningMessage = text
		case MessagePriorityStatus:
			s.StatusMessage = text
		}
		s.ForceRender = true
	}
}

// GetDisplayMessage returns the current highest-priority message to display (Tier 3 enhancement).
// Returns empty string if no message or if message has expired.
// REQUIRES: Caller must hold s.Mu lock (called only from renderProgressBox).
// MAY MUTATE: Clears expired messages as a side effect.
// The color/symbol parameters allow the caller to inject ANSI codes.
func (s *ProgressBoxState) GetDisplayMessage(cRed, cYellow, cCyan, cReset, sCross, sWarning, sInfo string) string {
	if s == nil {
		return ""
	}

	// Check if message has expired
	if !s.MessageExpiry.IsZero() && time.Now().After(s.MessageExpiry) {
		// Clear expired message
		s.StatusMessage = ""
		s.WarningMessage = ""
		s.ErrorMessage = ""
		s.MessagePriority = 0
		s.MessageExpiry = time.Time{}
		return ""
	}

	// Return highest priority message with symbol and color
	if s.ErrorMessage != "" {
		return fmt.Sprintf("%s%s %s%s", cRed, sCross, s.ErrorMessage, cReset)
	}
	if s.WarningMessage != "" {
		return fmt.Sprintf("%s%s %s%s", cYellow, sWarning, s.WarningMessage, cReset)
	}
	if s.StatusMessage != "" {
		return fmt.Sprintf("%s%s %s%s", cCyan, sInfo, s.StatusMessage, cReset)
	}

	return ""
}

// ResetForNewAlbum resets progress box state for a new album in a batch download.
// Preserves batch state, rclone settings, and start time across albums.
// Thread-safe: acquires mutex internally.
func (s *ProgressBoxState) ResetForNewAlbum(showTitle, showNumber string, trackTotal int, totalSize int64) {
	if s == nil {
		return
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()

	// Reset album-specific fields
	s.ShowTitle = showTitle
	s.ShowNumber = showNumber
	s.TrackTotal = trackTotal
	s.TrackNumber = 0
	s.TrackName = ""
	s.TrackFormat = ""

	// Reset progress percentages
	s.DownloadPercent = 0
	s.UploadPercent = 0
	s.ShowPercent = 0

	// Reset byte counters
	s.AccumulatedBytes = 0
	s.AccumulatedTracks = 0

	// Reset speed tracking
	s.SpeedHistory = nil
	s.UploadSpeedHistory = nil
	s.DownloadSpeed = ""
	s.UploadSpeed = ""
	s.Downloaded = ""
	s.Uploaded = ""
	s.DownloadETA = ""
	s.UploadETA = ""

	// Reset completion tracking
	s.SkippedTracks = 0
	s.ErrorTracks = 0
	s.IsComplete = false

	// Reset size totals
	s.DownloadTotal = ""
	s.UploadTotal = ""
	s.UploadTotalSet = false
	s.ShowDownloaded = ""
	s.ShowTotal = ""

	// Clear any messages from previous album
	s.StatusMessage = ""
	s.WarningMessage = ""
	s.ErrorMessage = ""
	s.MessagePriority = 0
	s.MessageExpiry = time.Time{}

	// Clear pause/cancel state for new album
	s.IsPaused = false
	s.IsCancelled = false
	s.ForceRender = true

	// Fields that are NOT reset (preserved across albums):
	// - RcloneEnabled (batch setting)
	// - BatchState (batch context)
	// - StartTime (batch start time, not album start)
	// - LinesDrawn (UI state managed by render function)
}

// GetQualityName returns a human-readable name for a format code (Tier 3 enhancement).
func GetQualityName(format int) string {
	switch format {
	case 1:
		return "ALAC 16/44.1"
	case 2:
		return "FLAC 16/44.1"
	case 3:
		return "MQA 24/48"
	case 4:
		return "360 Reality Audio"
	case 5:
		return "AAC 150kbps"
	default:
		return fmt.Sprintf("Format %d", format)
	}
}

// WriteCounter tracks download progress.
type WriteCounter struct {
	Total      int64
	TotalStr   string
	Downloaded int64
	Percentage int
	StartTime  int64
	OnProgress func(downloaded, total, speed int64)
}
