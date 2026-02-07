package main

import (
	"fmt"
	"time"
)

// SetMessage sets a temporary message with priority and duration (Tier 3 enhancement)
// Priority: 1=Status (cyan, ℹ), 2=Warning (yellow, ⚠), 3=Error (red, ✗)
// Duration: how long the message should be displayed before expiring
// Thread-safe: can be called concurrently with progress updates
func (s *ProgressBoxState) SetMessage(priority int, text string, duration time.Duration) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

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
	}
}

// GetDisplayMessage returns the current highest-priority message to display (Tier 3 enhancement)
// Returns empty string if no message or if message has expired
// REQUIRES: Caller must hold s.mu lock (called only from renderProgressBox)
// MAY MUTATE: Clears expired messages as a side effect
func (s *ProgressBoxState) GetDisplayMessage() string {
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
		return fmt.Sprintf("%s%s %s%s", colorRed, symbolCross, s.ErrorMessage, colorReset)
	}
	if s.WarningMessage != "" {
		return fmt.Sprintf("%s%s %s%s", colorYellow, symbolWarning, s.WarningMessage, colorReset)
	}
	if s.StatusMessage != "" {
		return fmt.Sprintf("%s%s %s%s", colorCyan, symbolInfo, s.StatusMessage, colorReset)
	}

	return ""
}

// getQualityName returns a human-readable name for a format code (Tier 3 enhancement)
func getQualityName(format int) string {
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

// ResetForNewAlbum resets progress box state for a new album in a batch download
// Preserves batch state, rclone settings, and start time across albums
// Thread-safe: acquires mutex internally
func (s *ProgressBoxState) ResetForNewAlbum(showTitle, showNumber string, trackTotal int, totalSize int64) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

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

	// Fields that are NOT reset (preserved across albums):
	// - RcloneEnabled (batch setting)
	// - BatchState (batch context)
	// - StartTime (batch start time, not album start)
	// - LinesDrawn (UI state managed by render function)
}
