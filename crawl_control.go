package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/term"
)

var ErrCrawlCancelled = errors.New("crawl cancelled by user")

type crawlController struct {
	active    atomic.Bool
	paused    atomic.Bool
	cancelled atomic.Bool
	restore   func()
	mu        sync.Mutex
}

var crawlerCtrl crawlController
var runtimeControlCache struct {
	mu        sync.Mutex
	lastRead  time.Time
	cachedVal RuntimeControl
}

func startCrawlHotkeysIfNeeded(urls []string) func() {
	if isReadOnlyCommand(urls) || os.Getenv(detachedEnvVar) == "1" {
		return func() {}
	}
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return func() {}
	}

	restore, err := enableHotkeyInput(fd)
	if err != nil {
		printWarning(fmt.Sprintf("Hotkeys unavailable: %v", err))
		return func() {}
	}

	crawlerCtrl.mu.Lock()
	crawlerCtrl.restore = restore
	crawlerCtrl.active.Store(true)
	crawlerCtrl.paused.Store(false)
	crawlerCtrl.cancelled.Store(false)
	crawlerCtrl.mu.Unlock()

	printInfo("Hotkeys: Shift-P pause/resume crawl | Shift-C cancel crawl")

	go func() {
		b := make([]byte, 1)
		for crawlerCtrl.active.Load() {
			n, readErr := os.Stdin.Read(b)
			if readErr != nil || n == 0 {
				return
			}
			switch b[0] {
			case 0x03: // Ctrl+C in raw mode
				fmt.Println("")
				requestCrawlCancel("Interrupted")
			case 'P':
				wasPaused := crawlerCtrl.paused.Load()
				crawlerCtrl.paused.Store(!wasPaused)
				_ = requestRuntimePause(!wasPaused)
				fmt.Println("")
				if wasPaused {
					printInfo("Crawl resumed")
				} else {
					printWarning("Crawl paused")
				}
			case 'C':
				if !crawlerCtrl.cancelled.Load() {
					crawlerCtrl.cancelled.Store(true)
					_ = requestRuntimeCancel()
					fmt.Println("")
					printWarning("Cancel requested. Stopping crawl...")
				}
				return
			}
		}
	}()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM)
		defer signal.Stop(sigCh)
		for range sigCh {
			fmt.Println("")
			requestCrawlCancel("Interrupted")
		}
	}()

	return func() {
		crawlerCtrl.active.Store(false)
		crawlerCtrl.paused.Store(false)
		crawlerCtrl.mu.Lock()
		defer crawlerCtrl.mu.Unlock()
		if crawlerCtrl.restore != nil {
			crawlerCtrl.restore()
			crawlerCtrl.restore = nil
		}
	}
}

func waitIfPausedOrCancelled() error {
	for {
		control := readRuntimeControlCached()
		controlPaused := control.Pause
		controlCancelled := control.Cancel

		if crawlerCtrl.cancelled.Load() || controlCancelled {
			// Tier 3: Update progress box with cancel indicator
			if pb := getCurrentProgressBox(); pb != nil {
				pb.Mu.Lock()
				pb.IsCancelled = true
				pb.Mu.Unlock()
				pb.SetMessage(MessagePriorityError, "Crawl cancelled by user", 10*time.Second)
				renderProgressBox(pb)
			}
			return ErrCrawlCancelled
		}

		if !crawlerCtrl.paused.Load() && !controlPaused {
			// Tier 3: Clear pause indicator when resuming
			if pb := getCurrentProgressBox(); pb != nil {
				pb.Mu.Lock()
				wasPaused := pb.IsPaused
				if wasPaused {
					pb.IsPaused = false
					// Force clear pause message by resetting priority first
					pb.MessagePriority = 0
				}
				pb.Mu.Unlock()
				if wasPaused {
					pb.SetMessage(MessagePriorityStatus, "Resumed", 2*time.Second)
					renderProgressBox(pb)
				}
			}
			break
		}

		// Tier 3: Set pause indicator with instructions (30 second timeout to allow other messages)
		if pb := getCurrentProgressBox(); pb != nil {
			pb.Mu.Lock()
			alreadyPaused := pb.IsPaused
			if !alreadyPaused {
				pb.IsPaused = true
			}
			pb.Mu.Unlock()
			if !alreadyPaused {
				pb.SetMessage(MessagePriorityWarning, "Paused - Press Shift-P to resume", 30*time.Second)
				renderProgressBox(pb)
			}
		}

		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

func isCrawlCancelledErr(err error) bool {
	return errors.Is(err, ErrCrawlCancelled)
}

func requestCrawlCancel(msg string) {
	if !crawlerCtrl.cancelled.Load() {
		crawlerCtrl.cancelled.Store(true)
		_ = requestRuntimeCancel()
	}
	printWarning(msg)
}

func readRuntimeControlCached() RuntimeControl {
	runtimeControlCache.mu.Lock()
	defer runtimeControlCache.mu.Unlock()

	cacheTTL := time.Second
	if crawlerCtrl.paused.Load() || runtimeControlCache.cachedVal.Pause {
		cacheTTL = 100 * time.Millisecond
	}

	now := time.Now()
	if now.Sub(runtimeControlCache.lastRead) < cacheTTL {
		return runtimeControlCache.cachedVal
	}
	runtimeControlCache.lastRead = now
	control, err := readRuntimeControl()
	if err == nil {
		runtimeControlCache.cachedVal = control
	}
	return runtimeControlCache.cachedVal
}
