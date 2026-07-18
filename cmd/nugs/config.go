package main

// Command adapters for configuration loading.

import "github.com/jmagar/nugs-cli/internal/config"

func getLoadedConfigPath() string { return config.LoadedConfigPath }

func promptForConfig() error                          { return config.PromptForConfig() }
func parseCfg() (*Config, error)                      { return config.ParseCfg() }
func resolveFfmpegBinary(cfg *Config) (string, error) { return config.ResolveFfmpegBinary(cfg) }
func normalizeCliAliases(urls []string) []string      { return config.NormalizeCliAliases(urls) }
