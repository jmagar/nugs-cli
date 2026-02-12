package model

import (
	"sync"
	"testing"
	"time"
)

func TestProgressBoxState_SetMessagePriority_Concurrent(t *testing.T) {
	state := &ProgressBoxState{}

	tests := []struct {
		name     string
		priority int
		text     string
	}{
		{name: "status", priority: MessagePriorityStatus, text: "status"},
		{name: "warning", priority: MessagePriorityWarning, text: "warning"},
		{name: "error", priority: MessagePriorityError, text: "error"},
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, tc := range tests {
		tc := tc
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			state.SetMessage(tc.priority, tc.text, time.Second)
		}()
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("concurrent SetMessage calls timed out")
	}

	state.Mu.Lock()
	defer state.Mu.Unlock()
	if state.MessagePriority != MessagePriorityError {
		t.Fatalf("expected highest priority message (%d), got %d", MessagePriorityError, state.MessagePriority)
	}
	if state.ErrorMessage != "error" {
		t.Fatalf("expected error message to win, got %q", state.ErrorMessage)
	}
}

func TestProgressBoxState_ShouldRender_ConcurrentAccess_NoDeadlock(t *testing.T) {
	state := &ProgressBoxState{RenderInterval: 0}

	const workers = 16
	const iterations = 200

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				state.Mu.Lock()
				state.TrackNumber = i + j
				state.Mu.Unlock()
				state.RequestRender()
				_ = state.ShouldRender(time.Now())
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("progress box concurrent render operations timed out")
	}
}
