// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package testutil provides common helper for tests.
package testutil

import (
	"context"
	"testing"
	"time"
)

const (
	// Arbitrary amount of time to let tests exit cleanly before main process terminates.
	timeoutGracePeriod = 10 * time.Second
)

// ContextWithDeadline returns context with will timeout before t.Deadline().
func ContextWithDeadline(t *testing.T) context.Context {
	t.Helper()

	deadline, ok := t.Deadline()
	if !ok {
		return context.Background()
	}

	ctx, cancel := context.WithDeadline(context.Background(), deadline.Truncate(timeoutGracePeriod))

	t.Cleanup(cancel)

	return ctx
}
