# life

`life` is a **minimal goroutine lifecycle convergence utility**.

This package **cannot** and should not control goroutine termination.
Goroutine shutdown and resource cleanup are the responsibility of the goroutines themselves.

## Problem

During program shutdown, Go code commonly runs into subtle and dangerous issues:

- New goroutines start after shutdown has begun
- `sync.WaitGroup.Add` races with `Wait` (undefined behavior)
- Shutdown signals are scattered and inconsistent
- Correctness relies on convention rather than enforcement

`life` solves **exactly one problem**:

> **After shutdown begins, no new goroutines are admitted,  
> and all previously started goroutines can be waited for  
> without violating `sync.WaitGroup` semantics.**

## Non-Goals (Important)

`life` **does NOT guarantee**:

- That goroutines will terminate
- That resources are released automatically
- Any timeout or force-stop behavior
- Any business-level cleanup logic

Goroutine termination is **cooperative**.

If a goroutine ignores the provided `context.Context` or blocks indefinitely,  
`ShutdownAndWait` will also block indefinitely.

This is an intentional design boundary.

## Design Principles

- **Cooperative cancellation** via `context.Context`
- **Hard admission boundary** after shutdown
- **Strict compliance** with `sync.WaitGroup` usage rules
- **Minimal abstraction**, no policy or strategy decisions

## Basic Usage

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/BinGo-Lab-Team/life"
)

func main() {
	l := life.NewLife(context.Background())

	// Start a lifecycle-managed goroutine
	if err := l.TryGo(func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				// Shutdown signal received
				log.Println("worker exiting")
				return
			default:
				// Simulate work
				time.Sleep(100 * time.Millisecond)
			}
		}
	}); err != nil {
		log.Println("failed to start goroutine:", err)
	}

	// Simulate program running
	time.Sleep(1 * time.Second)

	// Initiate shutdown and wait for goroutines to return
	l.ShutdownAndWait()
}
```

This guarantees the following order:

1. Stop admitting new goroutines
2. Cancel the root lifecycle context
3. Wait for all admitted goroutines to return

## API Overview

### `TryGo(func(ctx context.Context)) error`

- Attempts to start a lifecycle-managed goroutine
- Returns `ErrProgramShutting` after shutdown begins
- The function receives the shared root context

Unless `ShutdownAndWait` is called concurrently, `TryGo` is non-blocking
and returns immediately.

### `ShutdownAndWait()`

- Initiates shutdown
- Blocks new goroutine admission
- Waits for all started goroutines to return

### `LifeCtx() context.Context`

- Returns the root lifecycle context
- Useful for integration with external systems

## Intended Use Cases

- Server / daemon shutdown paths
- Systems that require strict goroutine lifecycle control
- Codebases that want to avoid `WaitGroup` shutdown races

## Design Notes

The implementation is intentionally compact but non-trivial.

`life` prioritizes **correctness and invariant enforcement**
 over readability or extensibility.

It is not a general concurrency framework.
 Policy decisions (timeouts, forced termination, retries)
 belong at a higher level.

## License

This project released under [MIT License](./LICENSE)
