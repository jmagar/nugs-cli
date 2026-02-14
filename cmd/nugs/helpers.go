package main

// Thin wrappers delegating to internal/helpers during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import "github.com/jmagar/nugs-cli/internal/helpers"

func handleErr(errText string, err error, _panic bool) { helpers.HandleErr(errText, err, _panic) }
func wasRunFromSrc() bool                              { return helpers.WasRunFromSrc() }
func getScriptDir() (string, error)                    { return helpers.GetScriptDir() }
func readTxtFile(path string) ([]string, error)        { return helpers.ReadTxtFile(path) }
func contains(lines []string, value string) bool       { return helpers.Contains(lines, value) }
func processUrls(urls []string) ([]string, error)      { return helpers.ProcessUrls(urls) }
func makeDirs(path string) error                       { return helpers.MakeDirs(path) }
func fileExists(path string) (bool, error)             { return helpers.FileExists(path) }
func sanitise(filename string) string                  { return helpers.Sanitise(filename) }
func validatePath(path string) error                   { return helpers.ValidatePath(path) }
func getVideoOutPath(cfg *Config) string               { return helpers.GetVideoOutPath(cfg) }
func getRcloneBasePath(cfg *Config, isVideo bool) string {
	return helpers.GetRcloneBasePath(cfg, isVideo)
}
func calculateLocalSize(localPath string) int64 { return helpers.CalculateLocalSize(localPath) }
func getOutPathForMedia(cfg *Config, mediaType MediaType) string {
	return helpers.GetOutPathForMedia(cfg, mediaType)
}
func getRclonePathForMedia(cfg *Config, mediaType MediaType) string {
	return helpers.GetRclonePathForMedia(cfg, mediaType)
}

func buildAlbumFolderName(artistName, containerInfo string, maxLen ...int) string {
	return helpers.BuildAlbumFolderName(artistName, containerInfo, maxLen...)
}
