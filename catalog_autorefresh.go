package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// Auto-Refresh Functions

// shouldAutoRefresh checks if the cache should be automatically refreshed
func shouldAutoRefresh(cfg *Config) (bool, error) {
	// Check if auto-refresh is enabled
	if !cfg.CatalogAutoRefresh {
		return false, nil
	}

	// Read cache metadata
	meta, err := readCacheMeta()
	if err != nil || meta == nil {
		// No cache or error reading - refresh
		return true, nil
	}

	// Parse configured timezone
	loc, err := time.LoadLocation(cfg.CatalogRefreshTimezone)
	if err != nil {
		return false, fmt.Errorf("invalid timezone %s: %w", cfg.CatalogRefreshTimezone, err)
	}

	// Get current time in configured timezone
	now := time.Now().In(loc)

	// Parse configured refresh time (e.g., "05:00")
	timePattern := regexp.MustCompile(`^(\d{2}):(\d{2})$`)
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

	// Validate time ranges
	if hour < 0 || hour > 23 {
		return false, fmt.Errorf("hour must be 00-23, got %d", hour)
	}
	if minute < 0 || minute > 59 {
		return false, fmt.Errorf("minute must be 00-59, got %d", minute)
	}

	// Create today's refresh time
	todayRefreshTime := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, loc)

	// Check interval
	switch cfg.CatalogRefreshInterval {
	case "daily":
		// Daily: refresh if current time is past refresh time today AND cache is from before today's refresh time
		if now.After(todayRefreshTime) && meta.LastUpdated.Before(todayRefreshTime) {
			return true, nil
		}
	case "weekly":
		// Weekly: refresh if current time is past refresh time today AND cache is more than 7 days old
		weekAgo := now.Add(-7 * 24 * time.Hour)
		if now.After(todayRefreshTime) && meta.LastUpdated.Before(weekAgo) {
			return true, nil
		}
	}

	return false, nil
}

// autoRefreshIfNeeded checks and performs auto-refresh if needed
func autoRefreshIfNeeded(cfg *Config) error {
	should, err := shouldAutoRefresh(cfg)
	if err != nil {
		// Log error but don't fail
		fmt.Fprintf(os.Stderr, "Auto-refresh check failed: %v\n", err)
		return nil
	}

	if !should {
		return nil
	}

	// Silently refresh
	fmt.Fprintf(os.Stderr, "Auto-refreshing catalog cache...\n")
	err = catalogUpdate("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Auto-refresh failed: %v\n", err)
	}

	return nil
}

// enableAutoRefresh enables auto-refresh with defaults
func enableAutoRefresh(cfg *Config) error {
	cfg.CatalogAutoRefresh = true

	// Set defaults if not already set
	if cfg.CatalogRefreshTime == "" {
		cfg.CatalogRefreshTime = "05:00"
	}
	if cfg.CatalogRefreshTimezone == "" {
		cfg.CatalogRefreshTimezone = "America/New_York"
	}
	if cfg.CatalogRefreshInterval == "" {
		cfg.CatalogRefreshInterval = "daily"
	}

	// Save config
	err := writeConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("✓ Auto-refresh enabled")
	fmt.Printf("  Time: %s %s\n", cfg.CatalogRefreshTime, cfg.CatalogRefreshTimezone)
	fmt.Printf("  Interval: %s\n", cfg.CatalogRefreshInterval)

	return nil
}

// disableAutoRefresh disables auto-refresh
func disableAutoRefresh(cfg *Config) error {
	cfg.CatalogAutoRefresh = false

	// Save config
	err := writeConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("✓ Auto-refresh disabled")
	return nil
}

// configureAutoRefresh prompts user to configure auto-refresh settings
func configureAutoRefresh(cfg *Config) error {
	reader := bufio.NewReader(os.Stdin)

	// Prompt for time
	fmt.Printf("Enter refresh time (HH:MM format, default 05:00): ")
	timeInput, _ := reader.ReadString('\n')
	timeInput = strings.TrimSpace(timeInput)
	if timeInput == "" {
		timeInput = "05:00"
	}

	// Validate time format
	timePattern := regexp.MustCompile(`^(\d{2}):(\d{2})$`)
	if !timePattern.MatchString(timeInput) {
		return fmt.Errorf("invalid time format: %s (expected HH:MM)", timeInput)
	}

	// Validate hour and minute ranges
	var hour, minute int
	fmt.Sscanf(timeInput, "%d:%d", &hour, &minute)
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return fmt.Errorf("invalid time: hour must be 00-23, minute must be 00-59")
	}

	// Prompt for timezone
	fmt.Printf("Enter timezone (e.g., America/New_York, UTC, America/Los_Angeles, default America/New_York): ")
	timezoneInput, _ := reader.ReadString('\n')
	timezoneInput = strings.TrimSpace(timezoneInput)
	if timezoneInput == "" {
		timezoneInput = "America/New_York"
	}

	// Validate timezone
	_, err := time.LoadLocation(timezoneInput)
	if err != nil {
		return fmt.Errorf("invalid timezone: %s", timezoneInput)
	}

	// Prompt for interval
	fmt.Printf("Enter refresh interval (daily or weekly, default daily): ")
	intervalInput, _ := reader.ReadString('\n')
	intervalInput = strings.TrimSpace(intervalInput)
	if intervalInput == "" {
		intervalInput = "daily"
	}

	// Validate interval
	if intervalInput != "daily" && intervalInput != "weekly" {
		return fmt.Errorf("invalid interval: %s (must be 'daily' or 'weekly')", intervalInput)
	}

	// Update config
	cfg.CatalogAutoRefresh = true
	cfg.CatalogRefreshTime = timeInput
	cfg.CatalogRefreshTimezone = timezoneInput
	cfg.CatalogRefreshInterval = intervalInput

	// Save config
	err = writeConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	printSuccess("Auto-refresh configured")
	printKeyValue("Time", fmt.Sprintf("%s %s", cfg.CatalogRefreshTime, cfg.CatalogRefreshTimezone), colorCyan)
	printKeyValue("Interval", cfg.CatalogRefreshInterval, colorCyan)

	return nil
}

// writeConfig writes the config back to the file it was loaded from
