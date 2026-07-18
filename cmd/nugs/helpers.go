package main

// Command adapters for internal helper operations.

import "github.com/jmagar/nugs-cli/internal/helpers"

func makeDirs(path string) error         { return helpers.MakeDirs(path) }
func sanitise(filename string) string    { return helpers.Sanitise(filename) }
func getVideoOutPath(cfg *Config) string { return helpers.GetVideoOutPath(cfg) }
func getRcloneBasePath(cfg *Config, isVideo bool) string {
	return helpers.GetRcloneBasePath(cfg, isVideo)
}

func buildAlbumFolderName(artistName, containerInfo string, maxLen ...int) string {
	return helpers.BuildAlbumFolderName(artistName, containerInfo, maxLen...)
}
