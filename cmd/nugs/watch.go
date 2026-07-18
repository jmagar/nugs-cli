package main

import (
	"context"
	"fmt"

	"github.com/jmagar/nugs-cli/internal/catalog"
	"github.com/jmagar/nugs-cli/internal/notify"
)

// watchAdd adds an artist to the watch list.
func watchAdd(cfg *Config, artistID string) error {
	return catalog.WatchAdd(cfg, artistID)
}

// watchRemove removes an artist from the watch list.
func watchRemove(cfg *Config, artistID string) error {
	return catalog.WatchRemove(cfg, artistID)
}

// watchList prints all watched artists.
func watchList(cfg *Config, jsonLevel string) error {
	return catalog.WatchList(cfg, jsonLevel)
}

// watchCheck updates the catalog and runs gap-fill for all watched artists.
func watchCheck(ctx context.Context, cfg *Config, streamParams *StreamParams, jsonLevel string, mediaFilter MediaType) error {
	deps := buildCatalogDeps()
	deps.Notify = notify.BuildNotifier(cfg.GotifyURL, cfg.GotifyToken)
	return catalog.WatchCheck(ctx, cfg, streamParams, jsonLevel, mediaFilter, deps)
}

// watchEnable generates systemd user units and enables the watch timer.
func watchEnable(cfg *Config) error {
	return catalog.WatchEnable(cfg)
}

// watchDisable stops and removes the watch timer and unit files.
func watchDisable() error {
	return catalog.WatchDisable()
}

// handleWatchCommand routes pre-auth "watch" subcommands (add/remove/list/enable/disable).
// Returns true if the command was handled. Returns false for "watch check" (post-auth).
func handleWatchCommand(ctx context.Context, cfg *Config, jsonLevel string) (bool, error) {
	if len(cfg.Urls) == 0 || cfg.Urls[0] != "watch" {
		return false, nil
	}

	if len(cfg.Urls) < 2 {
		printInfo("Usage: nugs watch add <artistID>")
		fmt.Println("       nugs watch remove <artistID>")
		fmt.Println("       nugs watch list")
		fmt.Println("       nugs watch check")
		fmt.Println("       nugs watch enable")
		fmt.Println("       nugs watch disable")
		return true, nil
	}

	subCmd := cfg.Urls[1]

	// "watch check" requires auth — defer to handleWatchCheckCommand.
	if subCmd == "check" {
		return false, nil
	}

	switch subCmd {
	case "add":
		if len(cfg.Urls) < 3 {
			printInfo("Usage: nugs watch add <artistID>")
			return true, nil
		}
		if err := watchAdd(cfg, cfg.Urls[2]); err != nil {
			return true, fmt.Errorf("watch add failed: %w", err)
		}
	case "remove":
		if len(cfg.Urls) < 3 {
			printInfo("Usage: nugs watch remove <artistID>")
			return true, nil
		}
		if err := watchRemove(cfg, cfg.Urls[2]); err != nil {
			return true, fmt.Errorf("watch remove failed: %w", err)
		}
	case "list":
		if err := watchList(cfg, jsonLevel); err != nil {
			return true, fmt.Errorf("watch list failed: %w", err)
		}
	case "enable":
		if err := watchEnable(cfg); err != nil {
			return true, fmt.Errorf("watch enable failed: %w", err)
		}
	case "disable":
		if err := watchDisable(); err != nil {
			return true, fmt.Errorf("watch disable failed: %w", err)
		}
	default:
		// Bare numeric ID shorthand: "nugs watch 1125" → "nugs watch add 1125"
		if err := watchAdd(cfg, subCmd); err != nil {
			return true, fmt.Errorf("watch add failed: %w", err)
		}
	}

	_ = ctx // ctx unused for pre-auth commands but kept for signature consistency
	return true, nil
}

// handleWatchCheckCommand routes post-auth "watch check". Returns true if handled.
func handleWatchCheckCommand(ctx context.Context, cfg *Config, streamParams *StreamParams, jsonLevel string) (bool, error) {
	if len(cfg.Urls) < 2 || cfg.Urls[0] != "watch" || cfg.Urls[1] != "check" {
		return false, nil
	}

	// Extract optional media modifier from remaining args.
	mediaFilter := MediaTypeUnknown
	if len(cfg.Urls) > 2 {
		mediaFilter, _ = parseMediaModifier(cfg.Urls[2:])
	}

	if err := watchCheck(ctx, cfg, streamParams, jsonLevel, mediaFilter); err != nil {
		return true, fmt.Errorf("watch check failed: %w", err)
	}
	return true, nil
}
