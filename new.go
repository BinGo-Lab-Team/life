package life

import "context"

// NewLife creates a new Life instance with a root lifecycle context.
//
// The returned Life derives its root context from the provided parent.
// Cancellation of either the parent context or the Life itself
// will transition the lifecycle into shutdown state.
//
// NewLife does not start any goroutines.
// Goroutines are only created via TryGo after construction.
//
// The caller is responsible for eventually calling ShutdownAndWait
// to release resources and wait for managed goroutines to return.
func NewLife(parent context.Context) *Life {
	ctx, cancel := context.WithCancel(parent)
	return &Life{
		ctx:    ctx,
		cancel: cancel,
	}
}
