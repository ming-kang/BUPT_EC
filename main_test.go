package main

import (
	"BUPT_EC/service"
	"testing"
)

func TestHTTPWriteTimeoutExceedsClassroomRefreshLimit(t *testing.T) {
	t.Parallel()
	if httpWriteTimeout <= service.ClassroomRefreshLimit {
		t.Fatalf("httpWriteTimeout (%v) must be greater than ClassroomRefreshLimit (%v)",
			httpWriteTimeout, service.ClassroomRefreshLimit)
	}
}

func TestGracefulShutdownTimeoutCoversClassroomRefresh(t *testing.T) {
	t.Parallel()
	if gracefulShutdownTimeout <= service.ClassroomRefreshLimit {
		t.Fatalf("gracefulShutdownTimeout (%v) must exceed ClassroomRefreshLimit (%v)",
			gracefulShutdownTimeout, service.ClassroomRefreshLimit)
	}
}
