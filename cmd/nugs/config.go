package main

// Config wrappers delegating to internal/config during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import "github.com/jmagar/nugs-cli/internal/config"

func getLoadedConfigPath() string { return config.LoadedConfigPath }

func promptForConfig() error                          { return config.PromptForConfig() }
func parseCfg() (*Config, error)                      { return config.ParseCfg() }
func resolveFfmpegBinary(cfg *Config) (string, error) { return config.ResolveFfmpegBinary(cfg) }
func normalizeCliAliases(urls []string) []string      { return config.NormalizeCliAliases(urls) }
