package model

import "context"

// UploadProgress describes one upload progress update from a storage backend.
type UploadProgress struct {
	Percent  int
	Speed    string
	Uploaded string
	Total    string
}

// UploadRequest describes what to upload and where to place it remotely.
type UploadRequest struct {
	LocalPath    string
	ArtistFolder string
	IsVideo      bool
}

// StorageHooks allows callers to subscribe to upload lifecycle events.
type StorageHooks struct {
	OnProgress          func(progress UploadProgress)
	OnPreUpload         func(totalBytes int64)
	OnComplete          func()
	OnDeleteAfterUpload func(localPath string)
}

// StorageProvider abstracts remote storage behavior for uploads and existence checks.
type StorageProvider interface {
	Upload(ctx context.Context, cfg *Config, req UploadRequest, hooks StorageHooks) error
	PathExists(ctx context.Context, cfg *Config, remotePath string, isVideo bool) (bool, error)
	ListArtistFolders(ctx context.Context, cfg *Config, artistFolder string, isVideo bool) (map[string]struct{}, error)
}
