package model

import "errors"

// Sentinel errors for download operations.
var (
	// ErrReleaseHasNoContent is returned when a release has no downloadable tracks or videos.
	ErrReleaseHasNoContent = errors.New("release has no tracks or videos")
)
