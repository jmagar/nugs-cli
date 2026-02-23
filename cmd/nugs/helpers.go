package main

// Thin wrappers delegating to internal/helpers during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import "github.com/jmagar/nugs-cli/internal/helpers"

func handleErr(errText string, err error, _panic bool) { helpers.HandleErr(errText, err, _panic) }
func makeDirs(path string) error                       { return helpers.MakeDirs(path) }
func sanitise(filename string) string                  { return helpers.Sanitise(filename) }
func getVideoOutPath(cfg *Config) string               { return helpers.GetVideoOutPath(cfg) }
func getRcloneBasePath(cfg *Config, isVideo bool) string {
	return helpers.GetRcloneBasePath(cfg, isVideo)
}

func buildAlbumFolderName(artistName, containerInfo string, maxLen ...int) string {
	return helpers.BuildAlbumFolderName(artistName, containerInfo, maxLen...)
}
