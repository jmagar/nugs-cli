// Package download implements the core download engine for audio and video content.
// It uses a Deps struct for callbacks to root-package functions that cannot be
// imported directly (ProgressBox rendering, crawl control, etc.).
package download

import "github.com/jmagar/nugs-cli/internal/model"

// Deps holds callbacks to root-package functions that the download package needs.
// These cannot be imported directly because they live in the root (main) package.
// The root package wires these up before calling any download functions.
type Deps struct {
	// WaitIfPausedOrCancelled checks if the crawl is paused or cancelled.
	// Returns ErrCrawlCancelled if cancelled, blocks while paused.
	WaitIfPausedOrCancelled func() error

	// IsCrawlCancelledErr checks if an error is a crawl cancellation error.
	IsCrawlCancelledErr func(error) bool

	// SetCurrentProgressBox sets the global progress box reference.
	SetCurrentProgressBox func(box *model.ProgressBoxState)

	// RenderProgressBox renders the progress box to the terminal.
	RenderProgressBox func(box *model.ProgressBoxState)

	// RenderCompletionSummary displays the final completion summary.
	RenderCompletionSummary func(box *model.ProgressBoxState)

	// UploadToRclone uploads a local path to rclone remote.
	UploadToRclone func(localPath, artistFolder string, cfg *model.Config, progressBox *model.ProgressBoxState, isVideo bool) error

	// RemotePathExists checks if a path exists on the rclone remote.
	RemotePathExists func(remotePath string, cfg *model.Config, isVideo bool) (bool, error)

	// PrintProgress renders a progress bar line for simple downloads.
	PrintProgress func(percentage int, speed, downloaded, total string)

	// UpdateSpeedHistory adds a new speed sample and maintains last 10 samples.
	UpdateSpeedHistory func(history []float64, newSpeed float64) []float64

	// CalculateETA calculates estimated time remaining based on speed history.
	CalculateETA func(speedHistory []float64, remaining int64) string
}
