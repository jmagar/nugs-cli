package rclone

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmagar/nugs-cli/internal/model"
)

func TestListArtistFoldersPreservesLastGoodBulkIndex(t *testing.T) {
	adapter := NewStorageAdapter()
	adapter.validatePath = func(string) error { return nil }
	adapter.commandContext = func(context.Context, string, ...string) *exec.Cmd { return exec.Command("true") }
	calls := 0
	adapter.outputCommand = func(*exec.Cmd) ([]byte, error) {
		calls++
		if calls == 1 {
			return []byte("show-one/\nshow-two/\n"), nil
		}
		return nil, errors.New("transient remote failure")
	}
	adapter.exitCode = func(error) (int, bool) { return 0, false }
	cfg := &model.Config{RcloneEnabled: true, RcloneRemote: "remote-" + t.Name()}
	first, err := adapter.ListArtistFolders(context.Background(), cfg, "artist", false)
	if err != nil || len(first) != 2 {
		t.Fatalf("first listing = %v, %v", first, err)
	}
	stale, err := adapter.ListArtistFolders(context.Background(), cfg, "artist", false)
	if err != nil || len(stale) != 2 {
		t.Fatalf("stale listing = %v, %v", stale, err)
	}
}

func TestRcloneProcessesAreGloballyBounded(t *testing.T) {
	adapter := NewStorageAdapter()
	adapter.validatePath = func(string) error { return nil }
	adapter.commandContext = func(context.Context, string, ...string) *exec.Cmd { return exec.Command("true") }
	var active atomic.Int32
	var peak atomic.Int32
	release := make(chan struct{})
	adapter.runCommand = func(*exec.Cmd) error {
		current := active.Add(1)
		for current > peak.Load() && !peak.CompareAndSwap(peak.Load(), current) {
		}
		<-release
		active.Add(-1)
		return nil
	}
	cfg := &model.Config{RcloneEnabled: true, RcloneRemote: "remote"}
	const calls = maxRcloneProcesses + 6
	var wg sync.WaitGroup
	for i := 0; i < calls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = adapter.PathExists(context.Background(), cfg, "artist/show", false)
		}()
	}
	deadline := time.Now().Add(time.Second)
	for peak.Load() < maxRcloneProcesses && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := peak.Load(); got > maxRcloneProcesses {
		t.Fatalf("peak rclone processes = %d, max = %d", got, maxRcloneProcesses)
	}
	close(release)
	wg.Wait()
}
