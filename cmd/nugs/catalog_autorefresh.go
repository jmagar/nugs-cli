package main

// Auto-refresh wrappers delegating to internal/catalog during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import (
	"context"

	"github.com/jmagar/nugs-cli/internal/catalog"
)

func shouldAutoRefresh(cfg *Config) (bool, error) {
	return catalog.ShouldAutoRefresh(cfg)
}

func autoRefreshIfNeeded(ctx context.Context, cfg *Config) error {
	return catalog.AutoRefreshIfNeeded(ctx, cfg, buildCatalogDeps())
}

func enableAutoRefresh(cfg *Config) error {
	return catalog.EnableAutoRefresh(cfg)
}

func disableAutoRefresh(cfg *Config) error {
	return catalog.DisableAutoRefresh(cfg)
}

func configureAutoRefresh(cfg *Config) error {
	return catalog.ConfigureAutoRefresh(cfg)
}
