package helpers

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmagar/nugs-cli/internal/model"
)

func TestSanitiseReservedPathComponents(t *testing.T) {
	for _, input := range []string{"", ".", "..", "  ..  "} {
		if got := Sanitise(input); got == "" || got == "." || got == ".." {
			t.Errorf("Sanitise(%q) = %q", input, got)
		}
	}
}

func TestSanitiseWindowsSeparators(t *testing.T) {
	got := Sanitise(`..\..\outside`)
	if strings.ContainsAny(got, `/\`) {
		t.Fatalf("Sanitise retained Windows traversal components: %q", got)
	}
}

func TestLocalShowPathConfinedToConfiguredRoot(t *testing.T) {
	root := t.TempDir()
	resolver := NewConfigPathResolver(&model.Config{OutPath: root})
	got := resolver.LocalShowPath(&model.AlbArtResp{ArtistName: "..", ContainerInfo: "../escape"}, model.MediaTypeAudio)
	rel, err := filepath.Rel(root, got)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("path escaped root: %q (rel %q, err %v)", got, rel, err)
	}
}

func TestLocalShowPathConfinesWindowsStyleMetadata(t *testing.T) {
	root := t.TempDir()
	resolver := NewConfigPathResolver(&model.Config{OutPath: root})
	got := resolver.LocalShowPath(&model.AlbArtResp{
		ArtistName:    `..\..\outside`,
		ContainerInfo: `..\..\show`,
	}, model.MediaTypeAudio)
	if strings.Contains(got, `\`) {
		t.Fatalf("path retained Windows separators: %q", got)
	}
	rel, err := filepath.Rel(root, got)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("path escaped root: %q (rel %q, err %v)", got, rel, err)
	}
}

func TestJoinWithinRootRejectsTraversal(t *testing.T) {
	if _, err := JoinWithinRoot(t.TempDir(), "..", "escape"); err == nil {
		t.Fatal("JoinWithinRoot accepted traversal")
	}
}

func TestRedactURLRemovesSignedQuery(t *testing.T) {
	got := RedactURL("https://media.example/track.m3u8?token=secret#frag")
	if strings.Contains(got, "secret") || strings.Contains(got, "?") || strings.Contains(got, "#") {
		t.Fatalf("RedactURL leaked sensitive URL data: %q", got)
	}
}
