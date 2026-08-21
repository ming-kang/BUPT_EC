package main

import (
	"BUPT_EC/service"
	"testing"
)

// The only handler that can wait on JW work is /api/get_data, and its wait is
// bounded by service.DefaultColdWaitTimeout (cold misses fail fast with 503 +
// Retry-After while the refresh continues in the background). The write
// timeout therefore needs headroom over that bound, not over the whole
// ClassroomRefreshLimit budget, which only governs background workers.
func TestHTTPWriteTimeoutExceedsColdWaitBound(t *testing.T) {
	t.Parallel()
	if httpWriteTimeout <= service.DefaultColdWaitTimeout {
		t.Fatalf("httpWriteTimeout (%v) must be greater than DefaultColdWaitTimeout (%v)",
			httpWriteTimeout, service.DefaultColdWaitTimeout)
	}
}

// Graceful shutdown cancels in-flight JW work promptly (lifecycle bridging),
// so its deadline needs to cover bounded handler writes plus drain margin —
// not the full refresh budget.
func TestGracefulShutdownTimeoutCoversColdWaitBound(t *testing.T) {
	t.Parallel()
	if gracefulShutdownTimeout <= service.DefaultColdWaitTimeout {
		t.Fatalf("gracefulShutdownTimeout (%v) must exceed DefaultColdWaitTimeout (%v)",
			gracefulShutdownTimeout, service.DefaultColdWaitTimeout)
	}
}
