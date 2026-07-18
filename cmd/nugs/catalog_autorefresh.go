package main

// Command adapters for catalog auto-refresh.

import (
	"context"

	"github.com/jmagar/nugs-cli/internal/catalog"
)

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
