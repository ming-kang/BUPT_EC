package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"BUPT_EC/service/model"
)

// TestColdMissWaitTimesOutThenConverges pins the bounded cold-wait contract:
// the first request on an empty cache fails fast with ErrRefreshWaitTimeout,
// the shared refresh it started keeps running (never cancelled), and the next
// call after the refresh completes serves fresh data.
func TestColdMissWaitTimesOutThenConverges(t *testing.T) {
	gate := make(chan struct{})
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			// Hold the refresh open until the test releases the gate.
			<-gate
			return []model.JWClassInfo{
				{NodeName: "1", NodeTime: "08:00-08:45", Classrooms: "教学实验综合楼-N104(229)"},
			}, nil
		},
	}
	svc := newTestServiceWithOptions(t, client, ClassroomServiceOptions{
		ColdWaitTimeout: 20 * time.Millisecond,
	})

	_, err := svc.GetTodayClassrooms(context.Background())
	if !errors.Is(err, ErrRefreshWaitTimeout) {
		t.Fatalf("GetTodayClassrooms() error = %v, want ErrRefreshWaitTimeout", err)
	}

	// Release the gate: the still-running refresh must complete and populate
	// the cache even though the first caller gave up waiting.
	close(gate)
	waitFor(t, 2*time.Second, func() bool {
		cached, ok := svc.getCachedTodayClassrooms()
		return ok && len(cached.Campuses) > 0
	})

	resp, err := svc.GetTodayClassrooms(context.Background())
	if err != nil {
		t.Fatalf("GetTodayClassrooms() after refresh = %v, want nil", err)
	}
	if resp.Stale {
		t.Fatal("expected fresh response after completed refresh")
	}
	if len(resp.Campuses) == 0 || len(resp.Campuses[0].Buildings) == 0 {
		t.Fatalf("unexpected refreshed payload: %#v", resp)
	}
}

// TestColdWaitTimeoutDefaultsToFiveSeconds locks the zero-value options
// behavior: operators and existing tests get the documented 5s bound without
// configuring anything.
func TestColdWaitTimeoutDefaultsToFiveSeconds(t *testing.T) {
	client := &mockJWClient{}
	svc := newTestServiceWithOptions(t, client, ClassroomServiceOptions{})
	if svc.coldWaitTimeout != defaultColdWaitTimeout {
		t.Fatalf("coldWaitTimeout = %v, want default %v", svc.coldWaitTimeout, defaultColdWaitTimeout)
	}

	negative := newTestServiceWithOptions(t, client, ClassroomServiceOptions{ColdWaitTimeout: -time.Second})
	if negative.coldWaitTimeout != defaultColdWaitTimeout {
		t.Fatalf("negative ColdWaitTimeout resolved to %v, want default %v", negative.coldWaitTimeout, defaultColdWaitTimeout)
	}
}
