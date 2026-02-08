package main

import (
	"testing"
	"time"
)

func TestProgressBoxShouldRenderThrottlesRapidUpdates(t *testing.T) {
	state := &ProgressBoxState{
		RenderInterval: 100 * time.Millisecond,
	}
	now := time.Unix(100, 0)

	if !state.ShouldRender(now) {
		t.Fatal("expected initial render to be allowed")
	}
	if state.ShouldRender(now.Add(50 * time.Millisecond)) {
		t.Fatal("expected render within interval to be throttled")
	}
	if !state.ShouldRender(now.Add(120 * time.Millisecond)) {
		t.Fatal("expected render after interval to be allowed")
	}
}

func TestProgressBoxShouldRenderForTrackChange(t *testing.T) {
	state := &ProgressBoxState{
		RenderInterval: 200 * time.Millisecond,
	}
	now := time.Unix(200, 0)

	if !state.ShouldRender(now) {
		t.Fatal("expected initial render to be allowed")
	}
	if state.ShouldRender(now.Add(50 * time.Millisecond)) {
		t.Fatal("expected render within interval to be throttled")
	}

	state.Mu.Lock()
	state.TrackNumber = 1
	state.Mu.Unlock()

	if !state.ShouldRender(now.Add(60 * time.Millisecond)) {
		t.Fatal("expected track change to force an immediate render")
	}
}

func TestProgressBoxShouldRenderWhenRequested(t *testing.T) {
	state := &ProgressBoxState{
		RenderInterval: 200 * time.Millisecond,
	}
	now := time.Unix(300, 0)

	if !state.ShouldRender(now) {
		t.Fatal("expected initial render to be allowed")
	}
	if state.ShouldRender(now.Add(50 * time.Millisecond)) {
		t.Fatal("expected render within interval to be throttled")
	}

	state.RequestRender()
	if !state.ShouldRender(now.Add(60 * time.Millisecond)) {
		t.Fatal("expected explicit render request to bypass throttling")
	}
}
