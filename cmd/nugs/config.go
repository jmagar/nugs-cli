package main

// Config wrappers delegating to internal/config during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import "github.com/jmagar/nugs-cli/internal/config"

// loadedConfigPath is accessed via getLoadedConfigPath / setLoadedConfigPath
// to maintain compatibility while delegating storage to config package.
func getLoadedConfigPath() string  { return config.LoadedConfigPath }
func setLoadedConfigPath(p string) { config.LoadedConfigPath = p }

var resolveRes = config.ResolveRes

func promptForConfig() error                          { return config.PromptForConfig() }
func parseCfg() (*Config, error)                      { return config.ParseCfg() }
func resolveFfmpegBinary(cfg *Config) (string, error) { return config.ResolveFfmpegBinary(cfg) }
func isShowCountFilterToken(s string) bool            { return config.IsShowCountFilterToken(s) }
func isMediaModifier(s string) bool                   { return config.IsMediaModifier(s) }
func normalizeCliAliases(urls []string) []string      { return config.NormalizeCliAliases(urls) }
func readConfig() (*Config, error)                    { return config.ReadConfig() }
func parseArgs() *Args                                { return config.ParseArgs() }
func writeConfig(cfg *Config) error                   { return config.WriteConfig(cfg) }
