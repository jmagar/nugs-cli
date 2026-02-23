package catalog

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jmagar/nugs-cli/internal/cache"
	"github.com/jmagar/nugs-cli/internal/config"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/ui"
)

var timePattern = regexp.MustCompile(`^(\d{2}):(\d{2})$`)

// ShouldAutoRefresh checks if the cache should be automatically refreshed.
func ShouldAutoRefresh(cfg *model.Config) (bool, error) {
	if !cfg.CatalogAutoRefresh {
		return false, nil
	}

	meta, err := cache.ReadCacheMeta()
	if err != nil || meta == nil {
		return true, nil
	}

	loc, err := time.LoadLocation(cfg.CatalogRefreshTimezone)
	if err != nil {
		return false, fmt.Errorf("invalid timezone %s: %w", cfg.CatalogRefreshTimezone, err)
	}

	now := time.Now().In(loc)

	matches := timePattern.FindStringSubmatch(cfg.CatalogRefreshTime)
	if matches == nil {
		return false, fmt.Errorf("invalid refresh time format: %s (expected HH:MM)", cfg.CatalogRefreshTime)
	}

	var hour, minute int
	if _, err := fmt.Sscanf(matches[1], "%d", &hour); err != nil {
		return false, fmt.Errorf("invalid hour in refresh time: %s", matches[1])
	}
	if _, err := fmt.Sscanf(matches[2], "%d", &minute); err != nil {
		return false, fmt.Errorf("invalid minute in refresh time: %s", matches[2])
	}

	if hour < 0 || hour > 23 {
		return false, fmt.Errorf("hour must be 00-23, got %d", hour)
	}
	if minute < 0 || minute > 59 {
		return false, fmt.Errorf("minute must be 00-59, got %d", minute)
	}

	todayRefreshTime := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, loc)

	switch cfg.CatalogRefreshInterval {
	case "hourly":
		if time.Since(meta.LastUpdated) >= time.Hour {
			return true, nil
		}
	case "daily":
		if now.After(todayRefreshTime) && meta.LastUpdated.Before(todayRefreshTime) {
			return true, nil
		}
	case "weekly":
		weekAgo := now.Add(-7 * 24 * time.Hour)
		if now.After(todayRefreshTime) && meta.LastUpdated.Before(weekAgo) {
			return true, nil
		}
	}

	return false, nil
}

// AutoRefreshIfNeeded checks and performs auto-refresh if needed.
func AutoRefreshIfNeeded(ctx context.Context, cfg *model.Config, deps *Deps) error {
	should, err := ShouldAutoRefresh(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Auto-refresh check failed: %v\n", err)
		return nil
	}

	if !should {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Auto-refreshing catalog cache...\n")
	err = CatalogUpdate(ctx, "", deps)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Auto-refresh failed: %v\n", err)
	}

	return nil
}

// EnableAutoRefresh enables auto-refresh with defaults.
func EnableAutoRefresh(cfg *model.Config) error {
	cfg.CatalogAutoRefresh = true

	if cfg.CatalogRefreshTime == "" {
		cfg.CatalogRefreshTime = "05:00"
	}
	if cfg.CatalogRefreshTimezone == "" {
		cfg.CatalogRefreshTimezone = "America/New_York"
	}
	if cfg.CatalogRefreshInterval == "" {
		cfg.CatalogRefreshInterval = "daily"
	}

	err := config.WriteConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("\u2713 Auto-refresh enabled")
	fmt.Printf("  Time: %s %s\n", cfg.CatalogRefreshTime, cfg.CatalogRefreshTimezone)
	fmt.Printf("  Interval: %s\n", cfg.CatalogRefreshInterval)

	return nil
}

// DisableAutoRefresh disables auto-refresh.
func DisableAutoRefresh(cfg *model.Config) error {
	cfg.CatalogAutoRefresh = false

	err := config.WriteConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("\u2713 Auto-refresh disabled")
	return nil
}

// ConfigureAutoRefresh prompts user to configure auto-refresh settings.
func ConfigureAutoRefresh(cfg *model.Config) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Enter refresh time (HH:MM format, default 05:00): ")
	timeInput, _ := reader.ReadString('\n')
	timeInput = strings.TrimSpace(timeInput)
	if timeInput == "" {
		timeInput = "05:00"
	}

	if !timePattern.MatchString(timeInput) {
		return fmt.Errorf("invalid time format: %s (expected HH:MM)", timeInput)
	}

	var hour, minute int
	if n, err := fmt.Sscanf(timeInput, "%d:%d", &hour, &minute); err != nil || n != 2 {
		return fmt.Errorf("failed to parse time: %s", timeInput)
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return fmt.Errorf("invalid time: hour must be 00-23, minute must be 00-59")
	}

	fmt.Printf("Enter timezone (e.g., America/New_York, UTC, America/Los_Angeles, default America/New_York): ")
	timezoneInput, _ := reader.ReadString('\n')
	timezoneInput = strings.TrimSpace(timezoneInput)
	if timezoneInput == "" {
		timezoneInput = "America/New_York"
	}

	_, err := time.LoadLocation(timezoneInput)
	if err != nil {
		return fmt.Errorf("invalid timezone: %s", timezoneInput)
	}

	fmt.Printf("Enter refresh interval (hourly, daily, or weekly, default daily): ")
	intervalInput, _ := reader.ReadString('\n')
	intervalInput = strings.TrimSpace(intervalInput)
	if intervalInput == "" {
		intervalInput = "daily"
	}

	if intervalInput != "hourly" && intervalInput != "daily" && intervalInput != "weekly" {
		return fmt.Errorf("invalid interval: %s (must be 'hourly', 'daily', or 'weekly')", intervalInput)
	}

	cfg.CatalogAutoRefresh = true
	cfg.CatalogRefreshTime = timeInput
	cfg.CatalogRefreshTimezone = timezoneInput
	cfg.CatalogRefreshInterval = intervalInput

	err = config.WriteConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	ui.PrintSuccess("Auto-refresh configured")
	ui.PrintKeyValue("Time", fmt.Sprintf("%s %s", cfg.CatalogRefreshTime, cfg.CatalogRefreshTimezone), ui.ColorCyan)
	ui.PrintKeyValue("Interval", cfg.CatalogRefreshInterval, ui.ColorCyan)

	return nil
}
