package service

// Cache day-boundary policy tests: cross-day rejection, completion-time
// stamping across midnight, and the Shanghai end-of-day helper.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"BUPT_EC/service/model"
)

func TestGetCachedTodayClassroomsRejectsCrossDayCache(t *testing.T) {
	svc := newTestService(t, &mockJWClient{})
	yesterday := time.Now().Add(-24 * time.Hour)
	seedCache(t, svc, &model.TodayClassrooms{
		Date:       yesterday.Format("2006-01-02"),
		ExpiresAt:  yesterday.Add(time.Hour),
		StaleUntil: yesterday.Add(time.Hour),
	})

	if cached, ok := svc.getCachedTodayClassrooms(); ok {
		t.Fatalf("expected cross-day cache miss, got %#v", cached)
	}

	now := time.Now()
	seedCache(t, svc, &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		ExpiresAt:  now.Add(time.Hour),
		StaleUntil: endOfDay(now),
	})

	if cached, ok := svc.getCachedTodayClassrooms(); !ok || cached.Date != now.Format("2006-01-02") {
		t.Fatalf("expected same-day cache hit, got %#v ok=%t", cached, ok)
	}
}

func TestDoRefreshStampsCacheAtCompletionAcrossMidnight(t *testing.T) {
	beforeMidnight := time.Date(2026, 7, 9, 23, 59, 50, 0, businessLocation)
	afterMidnight := time.Date(2026, 7, 10, 0, 0, 5, 0, businessLocation)

	clock := newFakeClock(beforeMidnight)
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			clock.Set(afterMidnight)
			return []model.JWClassInfo{{
				NodeName:   "1",
				NodeTime:   "08:00-08:45",
				Classrooms: fmt.Sprintf("教学实验综合楼-%s(10)", campusID),
			}}, nil
		},
	}
	svc := newTestServiceWithOptions(t, client, ClassroomServiceOptions{Clock: clock})

	result := svc.doRefreshTodayClassrooms(context.Background())
	if result.err != nil {
		t.Fatalf("doRefreshTodayClassrooms() error = %v", result.err)
	}
	if result.kind != refreshFull || result.value == nil {
		t.Fatalf("doRefreshTodayClassrooms() result = %#v, want full value", result)
	}
	resp := result.value

	wantDate := afterMidnight.Format("2006-01-02")
	if resp.Date != wantDate {
		t.Fatalf("Date = %q, want completion day %q", resp.Date, wantDate)
	}
	if !resp.UpdatedAt.Equal(afterMidnight) {
		t.Fatalf("UpdatedAt = %v, want %v", resp.UpdatedAt, afterMidnight)
	}
	if wantExp := afterMidnight.Add(classroomFreshTTL); !resp.ExpiresAt.Equal(wantExp) {
		t.Fatalf("ExpiresAt = %v, want %v", resp.ExpiresAt, wantExp)
	}
	wantStaleUntil := endOfDay(afterMidnight)
	if !resp.StaleUntil.Equal(wantStaleUntil) {
		t.Fatalf("StaleUntil = %v, want %v", resp.StaleUntil, wantStaleUntil)
	}

	cached, ok := svc.getCachedTodayClassrooms()
	if !ok {
		t.Fatal("expected completion-day cache hit")
	}
	if cached.Date != wantDate {
		t.Fatalf("cached Date = %q, want %q", cached.Date, wantDate)
	}
	if !cached.StaleUntil.Equal(wantStaleUntil) {
		t.Fatalf("cached StaleUntil = %v, want %v", cached.StaleUntil, wantStaleUntil)
	}
}

func TestEndOfDayIsNextMidnightShanghai(t *testing.T) {
	loc := businessLocation
	input := time.Date(2026, 7, 9, 15, 30, 0, 0, loc)
	got := endOfDay(input)
	want := time.Date(2026, 7, 10, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("endOfDay = %v, want %v", got, want)
	}

	// UTC evening that is still the same Shanghai calendar day should map to next Shanghai midnight.
	utc := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC) // 18:00 CST
	got = endOfDay(utc)
	want = time.Date(2026, 7, 10, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("endOfDay(UTC) = %v, want %v", got, want)
	}
}
