package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBuildAlbumFolderName_TruncatesByRunes(t *testing.T) {
	artist := "José González"
	container := strings.Repeat("漢", 200)

	name := buildAlbumFolderName(artist, container, 120)
	if !utf8.ValidString(name) {
		t.Fatalf("expected valid UTF-8 folder name")
	}
	if got := len([]rune(name)); got > 120 {
		t.Fatalf("expected <= 120 runes, got %d", got)
	}
}
