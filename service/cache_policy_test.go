package service

// Cache day-boundary policy tests: cross-day rejection, completion-time
// stamping across midnight, and the Shanghai end-of-day helper.

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
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

func TestTodayCacheEntryKeepsModelAndJSONInOneGeneration(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, businessLocation)
	clock := newFakeClock(now)
	svc := newTestServiceWithOptions(t, &mockJWClient{}, ClassroomServiceOptions{Clock: clock})
	today := &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		UpdatedAt:  now,
		ExpiresAt:  now.Add(time.Hour),
		StaleUntil: endOfDay(now),
		Campuses:   []model.CampusInfo{{ID: "01", Name: "西土城"}},
	}
	seedCache(t, svc, today)

	entry, ok := svc.getCachedTodayCacheEntryAt(now)
	if !ok {
		t.Fatal("expected same-day cache entry")
	}
	if entry.today != today {
		t.Fatalf("cache entry model = %p, want seeded model %p", entry.today, today)
	}
	dataJSON, ok := svc.GetCachedDataJSON()
	if !ok {
		t.Fatal("expected fresh cache JSON")
	}
	if dataJSON != entry.freshJSON {
		t.Fatal("fast-path JSON did not come from the loaded cache entry")
	}
	wantJSON, err := json.Marshal(classroomResponse(today, false, nil))
	if err != nil {
		t.Fatalf("marshal expected fresh JSON: %v", err)
	}
	if dataJSON != string(wantJSON) {
		t.Fatalf("cache entry JSON = %s, want model JSON %s", dataJSON, wantJSON)
	}
}

func TestTodayCacheEntryPublicationKeepsConcurrentGenerationsCoherent(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, businessLocation)
	clock := newFakeClock(now)
	svc := newTestServiceWithOptions(t, &mockJWClient{}, ClassroomServiceOptions{Clock: clock})
	publish := func(generation int) {
		svc.publishTodayCache(&model.TodayClassrooms{
			Date:       now.Format("2006-01-02"),
			UpdatedAt:  now,
			ExpiresAt:  now.Add(time.Hour),
			StaleUntil: endOfDay(now),
			Campuses:   []model.CampusInfo{{ID: fmt.Sprintf("generation-%d", generation)}},
		})
	}
	publish(0)

	const generations = 500
	published := make(chan struct{})
	loaded := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for generation := 1; generation <= generations; generation++ {
			publish(generation)
			published <- struct{}{}
			// Wait until the reader retains this entry, then publish the next
			// generation while the reader validates the retained one.
			<-loaded
		}
		close(published)
	}()
	go func() {
		defer wg.Done()
		for range published {
			entry, ok := svc.getCachedTodayCacheEntryAt(now)
			loaded <- struct{}{}
			if !ok {
				t.Error("expected same-day cache entry")
				continue
			}
			var decoded model.TodayClassrooms
			if err := json.Unmarshal([]byte(entry.freshJSON), &decoded); err != nil {
				t.Errorf("decode cache JSON: %v", err)
				continue
			}
			if len(entry.today.Campuses) != 1 || len(decoded.Campuses) != 1 || entry.today.Campuses[0].ID != decoded.Campuses[0].ID {
				t.Errorf("mixed cache generation: model=%#v JSON=%#v", entry.today.Campuses, decoded.Campuses)
			}
		}
	}()
	wg.Wait()
}

func TestGetCachedDataJSONRejectsCrossDayEntry(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, businessLocation)
	clock := newFakeClock(now)
	svc := newTestServiceWithOptions(t, &mockJWClient{}, ClassroomServiceOptions{Clock: clock})
	seedCache(t, svc, &model.TodayClassrooms{
		Date:       now.AddDate(0, 0, -1).Format("2006-01-02"),
		ExpiresAt:  now.Add(time.Hour),
		StaleUntil: now.Add(time.Hour),
	})

	if dataJSON, ok := svc.GetCachedDataJSON(); ok || dataJSON != "" {
		t.Fatalf("cross-day fast-path JSON = %q, %t; want empty, false", dataJSON, ok)
	}
}

func TestGetCachedDataJSONRejectsIneligibleEntry(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, businessLocation)
	cases := []struct {
		name   string
		mutate func(*model.TodayClassrooms)
	}{
		{
			name: "expired",
			mutate: func(today *model.TodayClassrooms) {
				today.ExpiresAt = now.Add(-time.Second)
			},
		},
		{
			name: "cached error",
			mutate: func(today *model.TodayClassrooms) {
				today.Error = &model.APIError{Type: "query", Message: "partial"}
			},
		},
		{
			name: "partial campuses",
			mutate: func(today *model.TodayClassrooms) {
				today.PartialCampuses = []string{"04"}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clock := newFakeClock(now)
			svc := newTestServiceWithOptions(t, &mockJWClient{}, ClassroomServiceOptions{Clock: clock})
			today := &model.TodayClassrooms{
				Date:       now.Format("2006-01-02"),
				ExpiresAt:  now.Add(time.Hour),
				StaleUntil: endOfDay(now),
			}
			tc.mutate(today)
			seedCache(t, svc, today)

			if dataJSON, ok := svc.GetCachedDataJSON(); ok || dataJSON != "" {
				t.Fatalf("ineligible fast-path JSON = %q, %t; want empty, false", dataJSON, ok)
			}
		})
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
