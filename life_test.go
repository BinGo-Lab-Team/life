package life_test

import (
	"context"
	"testing"
	"time"

	"github.com/BinGo-Lab-Team/life"
)

// TestTryGoBeforeShutdown verifies that TryGo succeeds
// and schedules a goroutine before shutdown begins.
func TestTryGoBeforeShutdown(t *testing.T) {
	l := life.NewLife(context.Background())

	done := make(chan struct{})

	if err := l.TryGo(func(ctx context.Context) {
		close(done)
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	<-done
	l.ShutdownAndWait()
}

// TestTryGoAfterShutdown verifies that TryGo is rejected
// once shutdown has completed.
func TestTryGoAfterShutdown(t *testing.T) {
	l := life.NewLife(context.Background())
	l.ShutdownAndWait()

	if err := l.TryGo(func(ctx context.Context) {}); err == nil {
		t.Fatal("expected error after shutdown")
	}
}

// TestShutdownWaitsForGoroutine verifies that ShutdownAndWait
// blocks until already-started goroutines return.
func TestShutdownWaitsForGoroutine(t *testing.T) {
	l := life.NewLife(context.Background())

	started := make(chan struct{})
	done := make(chan struct{})

	if err := l.TryGo(func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		close(done)
	}); err != nil {
		t.Fatal(err)
	}

	<-started
	go l.ShutdownAndWait()

	select {
	case <-done:
		// goroutine exited as expected
	case <-time.After(time.Second):
		t.Fatal("goroutine did not exit after shutdown")
	}
}

// TestConcurrentTryGoAndShutdown applies sustained pressure on TryGo
// while shutdown occurs, ensuring no panic or deadlock happens
// even if goroutines are being created concurrently.
func TestConcurrentTryGoAndShutdown(t *testing.T) {
	l := life.NewLife(context.Background())

	_ = l.TryGo(func(context.Context) {
		for {
			time.Sleep(10 * time.Millisecond)

			if err := l.TryGo(func(ctx context.Context) {
				for {
					select {
					case <-ctx.Done():
						return
					default:
						time.Sleep(100 * time.Millisecond)
					}
				}
			}); err != nil {
				// Shutdown in progress; exit pressure loop.
				break
			}
		}
	})

	time.Sleep(500 * time.Millisecond)
	l.ShutdownAndWait()
}
