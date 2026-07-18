package config

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestAtomicWriteFileConcurrentWriters(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "config.json")
	values := [][]byte{[]byte(`{"writer":1}`), []byte(`{"writer":2}`), []byte(`{"writer":3}`)}
	var wg sync.WaitGroup
	for _, value := range values {
		value := value
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := atomicWriteFile(target, value, 0600); err != nil {
				t.Errorf("atomicWriteFile: %v", err)
			}
		}()
	}
	wg.Wait()
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	valid := false
	for _, value := range values {
		valid = valid || bytes.Equal(got, value)
	}
	if !valid {
		t.Fatalf("partial or unexpected result: %q", got)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".config.json.*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("orphan temp files: %v", matches)
	}
}

func TestSyncDirectoryPlatformHook(t *testing.T) {
	if err := syncDirectory(t.TempDir()); err != nil {
		t.Fatalf("syncDirectory() error = %v", err)
	}
}
