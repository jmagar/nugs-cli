//go:build !linux

package catalog

import (
	"fmt"

	"github.com/jmagar/nugs-cli/internal/model"
)

// WatchEnable is not supported on non-Linux platforms.
func WatchEnable(_ *model.Config) error {
	return fmt.Errorf("watch enable requires systemd and is only supported on Linux")
}

// WatchDisable is not supported on non-Linux platforms.
func WatchDisable() error {
	return fmt.Errorf("watch disable requires systemd and is only supported on Linux")
}

// writeWatchUnitFiles is not supported on non-Linux platforms.
func writeWatchUnitFiles(_ *model.Config, _, _ string) error {
	return fmt.Errorf("systemd unit files require Linux")
}

// toSystemdDuration is not supported on non-Linux platforms.
func toSystemdDuration(_ string) (string, error) {
	return "", fmt.Errorf("systemd unit files require Linux")
}
