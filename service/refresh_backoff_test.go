package service

import (
	"testing"
	"time"
)

func TestTotalFailureBackoffLadder(t *testing.T) {
	want := []time.Duration{
		30 * time.Second,
		time.Minute,
		2 * time.Minute,
		5 * time.Minute,
		5 * time.Minute,
	}
	for i, expected := range want {
		got := totalFailureBackoff(i + 1)
		if got != expected {
			t.Fatalf("totalFailureBackoff(%d) = %v, want %v", i+1, got, expected)
		}
	}
	if totalFailureBackoff(0) != 30*time.Second {
		t.Fatalf("totalFailureBackoff(0) = %v, want 30s", totalFailureBackoff(0))
	}
}
