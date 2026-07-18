package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomicSyncsParentAndReplacesTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "state.json")
	if err := os.WriteFile(target, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(target, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("target = %q, want new", got)
	}
	if err := syncParentDirectory(dir); err != nil {
		t.Fatalf("syncParentDirectory() error = %v", err)
	}
}
