package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"BUPT_EC/logs"
	"BUPT_EC/service/model"
)

func TestStartClassroomRefreshPreservesLogIDWithoutRequestCancel(t *testing.T) {
	var (
		mu           sync.Mutex
		sawLogID     string
		refreshBegan = make(chan struct{})
		releaseQuery = make(chan struct{})
	)

	client := &mockJWClient{
		login: func(ctx context.Context, apiURL string) (string, error) {
			return "token", nil
		},
		fetchAPIURL: func(ctx context.Context) (string, error) {
			return DefaultAPIURL, nil
		},
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			mu.Lock()
			sawLogID = logs.GetLogIDFromContext(ctx)
			mu.Unlock()
			select {
			case <-refreshBegan:
			default:
				close(refreshBegan)
			}
			select {
			case <-releaseQuery:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			return []model.JWClassInfo{{Classrooms: "教学楼-101(40)", NodeTime: "08:00-09:35", NodeName: "1-2"}}, nil
		},
	}
	svc := newTestService(t, client)

	parent, cancel := context.WithCancel(logs.GenNewContext(context.Background()))
	wantLogID := logs.GetLogIDFromContext(parent)
	if wantLogID == "" {
		t.Fatal("test setup missing log_id")
	}

	attempt, started := svc.startClassroomRefresh(parent, svc.now())
	if !started || attempt == nil {
		t.Fatal("startClassroomRefresh should start a worker")
	}

	select {
	case <-refreshBegan:
	case <-time.After(2 * time.Second):
		t.Fatal("refresh did not begin")
	}

	// Cancel the initiator; the shared worker must keep running.
	cancel()
	close(releaseQuery)

	select {
	case <-attempt.done:
	case <-time.After(2 * time.Second):
		t.Fatal("refresh did not finish after initiator cancel")
	}

	mu.Lock()
	got := sawLogID
	mu.Unlock()
	if got != wantLogID {
		t.Fatalf("refresh worker log_id = %q, want initiator %q", got, wantLogID)
	}
	if attempt.result.err != nil {
		t.Fatalf("refresh error = %v", attempt.result.err)
	}
	if attempt.result.kind != refreshFull {
		t.Fatalf("refresh kind = %v, want full", attempt.result.kind)
	}
}
