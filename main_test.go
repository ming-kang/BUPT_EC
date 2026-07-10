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

func TestListenAddr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  string
		want string
	}{
		{name: "empty defaults to loopback", env: "", want: "127.0.0.1:8080"},
		{name: "explicit loopback", env: "127.0.0.1:8080", want: "127.0.0.1:8080"},
		{name: "explicit all interfaces", env: ":8080", want: ":8080"},
		{name: "explicit host port", env: "0.0.0.0:9090", want: "0.0.0.0:9090"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := listenAddr(tt.env); got != tt.want {
				t.Fatalf("listenAddr(%q) = %q, want %q", tt.env, got, tt.want)
			}
		})
	}
}
