// Package life provides a minimal and strictly-defined lifecycle controller
// for goroutines.
//
// The core goal of this package is NOT to "gracefully stop goroutines",
// but to enforce a set of hard concurrency invariants around goroutine
// admission and shutdown:
//
//  1. Goroutines may only be started via TryGo.
//  2. Once shutdown begins, all new TryGo calls are rejected.
//  3. ShutdownAndWait waits only for goroutines that actually return.
//  4. sync.WaitGroup.Add is never called concurrently with Wait.
//
// IMPORTANT DISCLAIMER:
//
// Life DOES NOT guarantee that goroutines will terminate.
// Goroutine termination is cooperative and entirely the responsibility
// of the caller. Goroutines must observe the provided context and return
// in a timely manner.
//
// If a goroutine ignores the context, blocks indefinitely, or leaks,
// ShutdownAndWait will block indefinitely as well.
//
// This package intentionally does not attempt to forcibly stop goroutines.
// It only provides correct accounting, ordering, and shutdown signaling.
package life

import (
	"context"
	"errors"
	"sync"
)

// ErrShuttingDown is returned by TryGo when the lifecycle
// has already entered shutdown state.
//
// Once this error is returned, no new goroutines will be started
// for the lifetime of the Life instance.
var ErrShuttingDown = errors.New("life shutting down")

// noCopy prevents accidental copying of Life.
//
// Copying Life would duplicate synchronization primitives
// (mutex, waitGroup) and violate lifecycle invariants.
type noCopy struct{}

// Life manages the lifecycle boundaries of a group of goroutines.
//
// Life guarantees *admission correctness* and *shutdown ordering*,
// but explicitly does NOT guarantee goroutine termination.
//
// All goroutines started via TryGo:
//
//   - Share the same root lifecycle context.
//   - Are counted exactly once in the internal WaitGroup.
//   - Will be waited for during shutdown *only if they return*.
//
// Life enforces a strict separation between:
//
//   - The "running" phase, where new goroutines may be admitted.
//   - The "shutdown" phase, where no new goroutines are allowed
//     and the system waits for existing ones to exit.
//
// The design ensures full compliance with sync.WaitGroup's contract:
// WaitGroup.Add is never called concurrently with Wait.
type Life struct {
	_ noCopy

	// ctx is the root lifecycle context shared by all goroutines.
	// Cancellation of this context marks the beginning of shutdown.
	ctx context.Context

	// cancel transitions the lifecycle into shutdown state.
	cancel context.CancelFunc

	// mu serializes the transition between goroutine admission
	// (TryGo) and shutdown waiting (ShutdownAndWait).
	//
	// TryGo holds mu as a read lock.
	// ShutdownAndWait takes the exclusive lock to block admission.
	mu sync.RWMutex

	// wg tracks all goroutines successfully started via TryGo.
	wg sync.WaitGroup
}

// LifeCtx returns the root lifecycle context.
//
// The returned context is canceled when ShutdownAndWait is called.
// Goroutines are expected to observe ctx.Done() and exit cooperatively.
func (l *Life) LifeCtx() context.Context { return l.ctx }

// TryGo attempts to start a new goroutine bound to the lifecycle.
//
// If the lifecycle is already shutting down, TryGo returns
// ErrShuttingDown and the function is not executed.
//
// The provided function receives the root lifecycle context.
// The function is expected to observe ctx.Done() and return.
//
// TryGo guarantees:
//
//   - No goroutine is started after shutdown begins.
//   - WaitGroup.Add is never called concurrently with Wait.
//   - Unless ShutdownAndWait is called concurrently,
//     TryGo is non-blocking and returns immediately.
//
// TryGo DOES NOT guarantee that the goroutine will exit;
// correct shutdown behavior requires cooperation from the function.
func (l *Life) TryGo(f func(life context.Context)) error {
	// Fast path: check shutdown state without taking locks.
	// This avoids unnecessary contention in the common case.
	select {
	case <-l.ctx.Done():
		return ErrShuttingDown
	default:
	}

	// Acquire read lock to synchronize with ShutdownAndWait.
	// Holding RLock guarantees that ShutdownAndWait cannot
	// proceed past its exclusive Lock until this section completes.
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Slow path: re-check shutdown state after acquiring the lock.
	// This closes the race where shutdown begins between the fast
	// path check and lock acquisition.
	select {
	case <-l.ctx.Done():
		return ErrShuttingDown
	default:
	}

	// At this point:
	//   - Shutdown has not begun.
	//   - ShutdownAndWait cannot proceed to Wait().
	// Therefore it is safe to call WaitGroup.Add.
	l.wg.Add(1)

	go func() {
		defer l.wg.Done()
		f(l.ctx)
	}()

	return nil
}

// ShutdownAndWait initiates shutdown and blocks until all
// lifecycle-managed goroutines have returned.
//
// The shutdown sequence is:
//
//  1. Cancel the root context, signaling all goroutines to stop.
//  2. Acquire the exclusive lock, blocking all concurrent TryGo calls
//     and waiting for in-flight TryGo calls to finish Add.
//  3. Call WaitGroup.Wait after no further Add calls are possible.
//
// NOTE:
//
// ShutdownAndWait does NOT forcibly stop goroutines.
// If any goroutine fails to return (e.g., ignores context
// cancellation or blocks indefinitely), this method will
// block indefinitely.
func (l *Life) ShutdownAndWait() {
	// Transition lifecycle into shutdown state.
	l.cancel()

	// Acquire exclusive lock to ensure that:
	//   - No new TryGo calls can proceed.
	//   - All in-progress TryGo calls have completed Add.
	l.mu.Lock()
	l.mu.Unlock()

	// Safe to wait: no further Add calls can occur.
	l.wg.Wait()
}
