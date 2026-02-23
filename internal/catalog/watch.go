package catalog

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/jmagar/nugs-cli/internal/cache"
	"github.com/jmagar/nugs-cli/internal/config"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/ui"
)

// WatchAdd adds an artist ID to the watch list in config.
// The ID must be a valid integer. Duplicates are silently ignored.
func WatchAdd(cfg *model.Config, artistID string) error {
	if _, err := strconv.Atoi(artistID); err != nil {
		return fmt.Errorf("invalid artist ID %q: must be a number", artistID)
	}

	// Deduplicate: only add if not already present.
	if slices.Contains(cfg.WatchedArtists, artistID) {
		name := resolveArtistName(artistID)
		if name != "" {
			ui.PrintInfo(fmt.Sprintf("%s (%s) is already in the watch list", artistID, name))
		} else {
			ui.PrintInfo(fmt.Sprintf("%s is already in the watch list", artistID))
		}
		return nil
	}

	cfg.WatchedArtists = append(cfg.WatchedArtists, artistID)

	if err := config.WriteConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	name := resolveArtistName(artistID)
	if name != "" {
		ui.PrintSuccess(fmt.Sprintf("Added %s (%s) to watch list", artistID, name))
	} else {
		ui.PrintSuccess(fmt.Sprintf("Added %s to watch list", artistID))
	}
	return nil
}

// WatchRemove removes an artist ID from the watch list in config.
// Returns an error if the ID is not currently watched.
func WatchRemove(cfg *model.Config, artistID string) error {
	if !slices.Contains(cfg.WatchedArtists, artistID) {
		return fmt.Errorf("artist %s is not in the watch list", artistID)
	}

	cfg.WatchedArtists = slices.DeleteFunc(cfg.WatchedArtists, func(id string) bool {
		return id == artistID
	})

	if err := config.WriteConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	name := resolveArtistName(artistID)
	if name != "" {
		ui.PrintSuccess(fmt.Sprintf("Removed %s (%s) from watch list", artistID, name))
	} else {
		ui.PrintSuccess(fmt.Sprintf("Removed %s from watch list", artistID))
	}
	return nil
}

// WatchList prints the current watch list with artist names where available.
func WatchList(cfg *model.Config, jsonLevel string) error {
	if len(cfg.WatchedArtists) == 0 {
		if jsonLevel != "" {
			return PrintJSON(map[string]any{"watchedArtists": []any{}})
		}
		fmt.Println("No artists in watch list. Add one with: nugs watch add <artistID>")
		return nil
	}

	// Build artistID (int) → name map from the containers index.
	nameByID := buildArtistNameMap()

	if jsonLevel != "" {
		entries := make([]map[string]any, len(cfg.WatchedArtists))
		for i, id := range cfg.WatchedArtists {
			entry := map[string]any{"artistID": id}
			idInt, err := strconv.Atoi(id)
			if err == nil {
				if name, ok := nameByID[idInt]; ok {
					entry["artistName"] = name
				}
			}
			entries[i] = entry
		}
		return PrintJSON(map[string]any{
			"watchedArtists": entries,
			"watchInterval":  watchIntervalOrDefault(cfg),
		})
	}

	ui.PrintHeader("Watched Artists")
	table := ui.NewTable([]ui.TableColumn{
		{Header: "ID", Width: 10, Align: "right"},
		{Header: "Artist", Width: 40, Align: "left"},
	})

	for _, id := range cfg.WatchedArtists {
		name := ""
		idInt, err := strconv.Atoi(id)
		if err == nil {
			name = nameByID[idInt]
		}
		if name == "" {
			name = "(name unavailable — run 'nugs catalog update' first)"
		}
		table.AddRow(id, name)
	}
	table.Print()

	interval := watchIntervalOrDefault(cfg)
	fmt.Printf("\n  Watch interval: %s%s%s\n", ui.ColorCyan, interval, ui.ColorReset)
	return nil
}

// WatchCheck updates the catalog then runs gap-fill for every watched artist.
// Artists are processed sequentially. Context cancellation stops between artists.
func WatchCheck(ctx context.Context, cfg *model.Config, streamParams *model.StreamParams, jsonLevel string, mediaFilter model.MediaType, deps *Deps) error {
	if len(cfg.WatchedArtists) == 0 {
		if jsonLevel == "" {
			ui.PrintInfo("No artists in watch list. Add one with: nugs watch add <artistID>")
		}
		return nil
	}

	// Silently update the catalog first. Pass jsonLevel so CatalogUpdate suppresses
	// its human-readable table output when the caller expects JSON on stdout.
	if err := CatalogUpdate(ctx, jsonLevel, deps); err != nil {
		// Non-fatal: proceed with potentially stale catalog.
		if jsonLevel == "" {
			ui.PrintWarning(fmt.Sprintf("Catalog update failed (continuing with cached data): %v", err))
		}
	}

	totalDownloaded := 0
	totalFailed := 0
	var artistErrors []string

	for _, artistID := range cfg.WatchedArtists {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if jsonLevel == "" {
			name := resolveArtistName(artistID)
			if name != "" {
				ui.PrintHeader(fmt.Sprintf("Checking %s (%s)", artistID, name))
			} else {
				ui.PrintHeader(fmt.Sprintf("Checking artist %s", artistID))
			}
		}

		result, err := CatalogGapsFill(ctx, artistID, cfg, streamParams, jsonLevel, mediaFilter, deps)
		if err != nil {
			if jsonLevel == "" {
				ui.PrintWarning(fmt.Sprintf("Gap fill failed for artist %s: %v", artistID, err))
			}
			artistErrors = append(artistErrors, fmt.Sprintf("%s: %v", artistID, err))
			// Continue to next artist even if one fails.
		} else {
			totalDownloaded += result.Downloaded
			totalFailed += result.Failed
			if len(cfg.WatchedArtists) > 1 {
				sendArtistUpdate(ctx, deps.Notify, result, artistID)
			}
			if result.Interrupted {
				break // user cancelled mid-download; don't process remaining artists
			}
		}
	}

	sendWatchSummary(ctx, deps.Notify, totalDownloaded, totalFailed, artistErrors)
	return nil
}

// sendArtistUpdate fires a per-artist notification immediately after that artist's gap-fill
// completes with downloads. Only called when multiple artists are being watched —
// single-artist runs rely on the final sendWatchSummary to avoid a redundant double-ping.
func sendArtistUpdate(ctx context.Context, notify func(ctx context.Context, title, message string, priority int) error, result GapFillResult, artistID string) {
	if notify == nil || result.Downloaded == 0 {
		return
	}
	name := result.ArtistName
	if name == "" {
		name = artistID
	}
	msg := fmt.Sprintf("%d new show(s) downloaded for %s", result.Downloaded, name)
	_ = notify(ctx, "Nugs Watch", msg, 5)
}

// sendWatchSummary fires a single Gotify notification summarising the watch check outcome.
// Sends nothing if notify is nil or there is nothing to report (all up-to-date, no errors).
func sendWatchSummary(ctx context.Context, notify func(ctx context.Context, title, message string, priority int) error, downloaded, failed int, errs []string) {
	if notify == nil {
		return
	}

	switch {
	case downloaded > 0:
		msg := fmt.Sprintf("%d new show(s) downloaded", downloaded)
		if failed > 0 {
			msg += fmt.Sprintf(", %d failed", failed)
		}
		if len(errs) > 0 {
			msg += fmt.Sprintf(", %d artist error(s)", len(errs))
		}
		_ = notify(ctx, "Nugs Watch", msg, 5)

	case failed > 0 || len(errs) > 0:
		var parts []string
		if failed > 0 {
			parts = append(parts, fmt.Sprintf("%d download failure(s)", failed))
		}
		parts = append(parts, errs...)
		_ = notify(ctx, "Nugs Watch Error", strings.Join(parts, "\n"), 7)

	// Nothing to report: all artists up-to-date, no errors. Stay silent.
	}
}

// buildArtistNameMap reads the containers index and returns a map of artistID → artistName.
// Returns an empty map if the cache is unavailable.
func buildArtistNameMap() map[int]string {
	idx, err := cache.ReadContainersIndex()
	if err != nil || idx == nil {
		return map[int]string{}
	}
	names := make(map[int]string, len(idx.Containers))
	for _, entry := range idx.Containers {
		if _, ok := names[entry.ArtistID]; !ok {
			names[entry.ArtistID] = entry.ArtistName
		}
	}
	return names
}

// resolveArtistName looks up an artist name from the containers index cache.
// Returns an empty string if the cache is unavailable or the artist is not found.
func resolveArtistName(artistID string) string {
	idInt, err := strconv.Atoi(artistID)
	if err != nil {
		return ""
	}
	names := buildArtistNameMap()
	return names[idInt]
}

// watchIntervalOrDefault returns cfg.WatchInterval or the default "1h".
func watchIntervalOrDefault(cfg *model.Config) string {
	if cfg.WatchInterval != "" {
		return cfg.WatchInterval
	}
	return "1h"
}
