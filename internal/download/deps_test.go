package download

import (
	"context"
	"testing"

	"github.com/jmagar/nugs-cli/internal/model"
)

type fakeStorageProvider struct {
	uploadCalls     int
	lastUpload      model.UploadRequest
	pathExistsCalls int
	lastPath        string
	lastPathIsVideo bool
	pathExists      bool
}

func (f *fakeStorageProvider) Upload(_ context.Context, _ *model.Config, req model.UploadRequest, _ model.StorageHooks) error {
	f.uploadCalls++
	f.lastUpload = req
	return nil
}

func (f *fakeStorageProvider) PathExists(_ context.Context, _ *model.Config, remotePath string, isVideo bool) (bool, error) {
	f.pathExistsCalls++
	f.lastPath = remotePath
	f.lastPathIsVideo = isVideo
	return f.pathExists, nil
}

func (f *fakeStorageProvider) ListArtistFolders(_ context.Context, _ *model.Config, _ string, _ bool) (map[string]struct{}, error) {
	return map[string]struct{}{}, nil
}

func TestDepsUploadPathUsesStorageProviderWhenLegacyCallbackMissing(t *testing.T) {
	storage := &fakeStorageProvider{}
	deps := &Deps{Storage: storage}

	err := deps.UploadPath("/tmp/album", "artist", &model.Config{}, nil, false)
	if err != nil {
		t.Fatalf("upload returned error: %v", err)
	}
	if storage.uploadCalls != 1 {
		t.Fatalf("upload calls = %d, want 1", storage.uploadCalls)
	}
	if storage.lastUpload.LocalPath != "/tmp/album" {
		t.Fatalf("local path = %q", storage.lastUpload.LocalPath)
	}
	if storage.lastUpload.ArtistFolder != "artist" {
		t.Fatalf("artist folder = %q", storage.lastUpload.ArtistFolder)
	}
}

func TestDepsCheckRemotePathExistsUsesStorageProviderWhenLegacyCallbackMissing(t *testing.T) {
	storage := &fakeStorageProvider{pathExists: true}
	deps := &Deps{Storage: storage}

	exists, err := deps.CheckRemotePathExists("artist/show", &model.Config{}, true)
	if err != nil {
		t.Fatalf("path exists returned error: %v", err)
	}
	if !exists {
		t.Fatal("expected exists=true")
	}
	if storage.pathExistsCalls != 1 {
		t.Fatalf("path exists calls = %d, want 1", storage.pathExistsCalls)
	}
	if storage.lastPath != "artist/show" {
		t.Fatalf("last path = %q", storage.lastPath)
	}
	if !storage.lastPathIsVideo {
		t.Fatal("expected isVideo=true")
	}
}
