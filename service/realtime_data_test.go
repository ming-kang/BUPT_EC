package service

// GetTodayClassrooms main-flow tests (fresh serve, stale-while-refresh,
// broadcast to waiters, warmup sharing, backoff, partial-error retry and
// recovery) plus the queryAll fixture and concurrency tests.

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"BUPT_EC/service/model"
)

func TestGetTodayClassroomsReturnsFreshCacheWithoutJWQuery(t *testing.T) {
	var queryCalls atomic.Int32
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			queryCalls.Add(1)
			return nil, errors.New("fresh cache should not query JW")
		},
	}
	classroomServiceUnderTest := newTestService(t, client)

	now := time.Now().In(businessLocation)
	seedCache(t, classroomServiceUnderTest, &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		UpdatedAt:  now,
		ExpiresAt:  now.Add(time.Minute),
		StaleUntil: endOfDay(now),
		Campuses: []model.CampusInfo{
			{ID: "cached", Name: "cached campus"},
		},
	})

	response, err := classroomServiceUnderTest.GetTodayClassrooms(context.Background())
	if err != nil {
		t.Fatalf("GetTodayClassrooms() error = %v", err)
	}
	if response.Stale {
		t.Fatal("expected fresh cache response to be non-stale")
	}
	if response.Error != nil {
		t.Fatalf("expected fresh cache response without API error, got %#v", response.Error)
	}
	if len(response.Campuses) != 1 || response.Campuses[0].ID != "cached" {
		t.Fatalf("unexpected cached campuses: %#v", response.Campuses)
	}
	if queryCalls.Load() != 0 {
		t.Fatalf("expected no JW query for fresh cache, got %d", queryCalls.Load())
	}
}

func TestQueryAllBuildsTodayClassroomsFromJWFixture(t *testing.T) {
	rowsByCampus := map[string][]model.JWClassInfo{
		"01": {
			{
				NodeName:   "2",
				NodeTime:   "09:00-09:45",
				Classrooms: "教学实验综合楼-N104(229),未来学习大楼-202-203(60),教学实验综合楼-N104(229)",
			},
			{
				NodeName:   "1",
				NodeTime:   "08:00-08:45",
				Classrooms: "教学实验综合楼-N104(229)",
			},
		},
		"04": {
			{
				NodeName:   "1",
				NodeTime:   "08:00-08:45",
				Classrooms: "沙河教学楼-S101(40)",
			},
		},
	}
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			rows, ok := rowsByCampus[campusID]
			if !ok {
				return nil, fmt.Errorf("unexpected campus query: %s", campusID)
			}
			return rows, nil
		},
	}
	classroomServiceUnderTest := newTestServiceWithOverride(t, client, "fixture-token")

	response, err := classroomServiceUnderTest.queryAll(context.Background())
	if err != nil {
		t.Fatalf("queryAll() error = %v", err)
	}
	if response.Stale {
		t.Fatal("expected refreshed fixture response to be non-stale")
	}
	if response.Error != nil {
		t.Fatalf("expected refreshed fixture response without API error, got %#v", response.Error)
	}
	if len(response.Campuses) != 2 {
		t.Fatalf("expected two campuses, got %#v", response.Campuses)
	}

	xituchengCampus := requireCampusByID(t, response.Campuses, "01")
	if xituchengCampus.Name != "西土城" {
		t.Fatalf("campus 01 name = %q, want 西土城", xituchengCampus.Name)
	}
	expectedNodes := []model.NodeInfo{
		{Node: 1, Time: "08:00-08:45", RoomCount: 1},
		{Node: 2, Time: "09:00-09:45", RoomCount: 2},
	}
	if !reflect.DeepEqual(xituchengCampus.Nodes, expectedNodes) {
		t.Fatalf("campus 01 nodes = %#v, want %#v", xituchengCampus.Nodes, expectedNodes)
	}

	standardRoom := requireRoomByDisplayName(t, requireBuildingByName(t, xituchengCampus.Buildings, "教学实验综合楼").Rooms, "教学实验综合楼-N104")
	if standardRoom.Name != "N104" || standardRoom.Capacity != 229 {
		t.Fatalf("unexpected standard room metadata: %#v", standardRoom)
	}
	if !reflect.DeepEqual(standardRoom.FreeNodes, []int{1, 2}) {
		t.Fatalf("standard room free_nodes = %#v, want [1 2]", standardRoom.FreeNodes)
	}
	expectedStandardFreeTimes := []model.FreeTime{
		{Node: 1, Time: "08:00-08:45"},
		{Node: 2, Time: "09:00-09:45"},
	}
	if !reflect.DeepEqual(standardRoom.FreeTimes, expectedStandardFreeTimes) {
		t.Fatalf("standard room free_times = %#v, want %#v", standardRoom.FreeTimes, expectedStandardFreeTimes)
	}

	mergedRoom := requireRoomByDisplayName(t, requireBuildingByName(t, xituchengCampus.Buildings, "未来学习大楼").Rooms, "未来学习大楼-202-203")
	if mergedRoom.Name != "202-203" || mergedRoom.Capacity != 60 {
		t.Fatalf("unexpected merged room metadata: %#v", mergedRoom)
	}
	if !reflect.DeepEqual(mergedRoom.FreeNodes, []int{2}) {
		t.Fatalf("merged room free_nodes = %#v, want [2]", mergedRoom.FreeNodes)
	}

	shaheCampus := requireCampusByID(t, response.Campuses, "04")
	if shaheCampus.Name != "沙河" {
		t.Fatalf("campus 04 name = %q, want 沙河", shaheCampus.Name)
	}
	shaheRoom := requireRoomByDisplayName(t, requireBuildingByName(t, shaheCampus.Buildings, "沙河教学楼").Rooms, "沙河教学楼-S101")
	if shaheRoom.Capacity != 40 || !reflect.DeepEqual(shaheRoom.FreeNodes, []int{1}) {
		t.Fatalf("unexpected shahe room shape: %#v", shaheRoom)
	}
}

func TestGetTodayClassroomsReturnsStaleWhileRefreshContinues(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	var calls atomic.Int32
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			calls.Add(1)
			startedOnce.Do(func() { close(started) })
			select {
			case <-release:
				return []model.JWClassInfo{{
					NodeName:   "1",
					NodeTime:   "08:00-08:45",
					Classrooms: fmt.Sprintf("教学实验综合楼-%s(10)", campusID),
				}}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}
	svc := newTestService(t, client)

	now := time.Now().In(businessLocation)
	seedCache(t, svc, &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		UpdatedAt:  now.Add(-time.Hour),
		ExpiresAt:  now.Add(-time.Minute),
		StaleUntil: endOfDay(now),
		Campuses: []model.CampusInfo{
			{ID: "cached", Name: "cached"},
		},
	})

	start := time.Now()
	resp, err := svc.GetTodayClassrooms(context.Background())
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("GetTodayClassrooms() error = %v", err)
	}
	if !resp.Stale {
		t.Fatal("expected stale response while refresh is still running")
	}
	if resp.Error != nil {
		t.Fatalf("expected no stale error before refresh fails, got %#v", resp.Error)
	}
	if elapsed > time.Second {
		t.Fatalf("stale response took too long: %s", elapsed)
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected background refresh to start")
	}
	close(release)
	waitFor(t, time.Second, func() bool {
		cached, ok := svc.getCachedTodayClassrooms()
		return ok && len(cached.Campuses) == 2
	})
	if calls.Load() != 2 {
		t.Fatalf("expected one query per campus, got %d", calls.Load())
	}
}

func TestGetTodayClassroomsBroadcastsRefreshResultToConcurrentWaiters(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			startedOnce.Do(func() { close(started) })
			select {
			case <-release:
				return []model.JWClassInfo{{
					NodeName:   "1",
					NodeTime:   "08:00-08:45",
					Classrooms: fmt.Sprintf("教学实验综合楼-%s(10)", campusID),
				}}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}
	svc := newTestService(t, client)

	const waiters = 8
	errs := make(chan error, waiters)
	var wg sync.WaitGroup
	wg.Add(waiters)
	for i := 0; i < waiters; i++ {
		go func() {
			defer wg.Done()
			resp, err := svc.GetTodayClassrooms(context.Background())
			if err != nil {
				errs <- err
				return
			}
			if resp == nil || resp.Stale || len(resp.Campuses) != 2 {
				errs <- fmt.Errorf("unexpected response: %#v", resp)
			}
		}()
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected shared refresh to start")
	}
	close(release)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestGetTodayClassroomsSharesWarmupRefreshResult(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			startedOnce.Do(func() { close(started) })
			select {
			case <-release:
				return []model.JWClassInfo{{
					NodeName:   "1",
					NodeTime:   "08:00-08:45",
					Classrooms: fmt.Sprintf("教学实验综合楼-%s(10)", campusID),
				}}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}
	svc := newTestService(t, client)

	runCtx, runCancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- svc.Run(runCtx) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected warmup refresh to start")
	}

	errCh := make(chan error, 1)
	go func() {
		resp, err := svc.GetTodayClassrooms(context.Background())
		if err != nil {
			errCh <- err
			return
		}
		if resp == nil || resp.Stale || len(resp.Campuses) != 2 {
			errCh <- fmt.Errorf("unexpected response: %#v", resp)
			return
		}
		errCh <- nil
	}()

	close(release)
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("request did not receive warmup refresh result")
	}
	runCancel()
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not return after cancel")
	}
}

func TestGetTodayClassroomsBacksOffAfterStaleRefreshFailure(t *testing.T) {
	var calls atomic.Int32
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			calls.Add(1)
			return nil, newJWError(jwErrorQuery, "jw query", nil, "upstream failed")
		},
	}
	svc := newTestService(t, client)

	now := time.Now().In(businessLocation)
	seedCache(t, svc, &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		UpdatedAt:  now.Add(-time.Hour),
		ExpiresAt:  now.Add(-time.Minute),
		StaleUntil: endOfDay(now),
		Campuses:   []model.CampusInfo{{ID: "cached", Name: "cached"}},
	})

	resp, err := svc.GetTodayClassrooms(context.Background())
	if err != nil {
		t.Fatalf("GetTodayClassrooms() error = %v", err)
	}
	if !resp.Stale || resp.Error == nil {
		t.Fatalf("expected stale response with refresh error, got %#v", resp)
	}
	firstCalls := calls.Load()
	if firstCalls == 0 {
		t.Fatal("expected refresh attempt")
	}

	resp, err = svc.GetTodayClassrooms(context.Background())
	if err != nil {
		t.Fatalf("GetTodayClassrooms() second error = %v", err)
	}
	if !resp.Stale || resp.Error == nil {
		t.Fatalf("expected stale backoff response with last error, got %#v", resp)
	}
	if got := calls.Load(); got != firstCalls {
		t.Fatalf("expected refresh backoff to suppress new calls, got %d want %d", got, firstCalls)
	}
}

func TestQueryAllQueriesCampusesConcurrently(t *testing.T) {
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			select {
			case <-time.After(300 * time.Millisecond):
				return []model.JWClassInfo{{
					NodeName:   "1",
					NodeTime:   "08:00-08:45",
					Classrooms: fmt.Sprintf("教学实验综合楼-%s(10)", campusID),
				}}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}
	svc := newTestService(t, client)

	start := time.Now()
	resp, err := svc.queryAll(context.Background())
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("queryAll() error = %v", err)
	}
	if len(resp.Campuses) != 2 {
		t.Fatalf("expected two campuses, got %d", len(resp.Campuses))
	}
	if elapsed > 550*time.Millisecond {
		t.Fatalf("expected concurrent campus queries, took %s", elapsed)
	}
}

// Partial-campus error must not freeze retries for the full fresh TTL: a later
// Get inside the old ExpiresAt window should still kick a background refresh
// (subject to single-flight + 30s backoff).
func TestGetTodayClassroomsRetriesPartialErrorWithinFreshTTL(t *testing.T) {
	var mu sync.Mutex
	var shaheCalls int
	var allCalls int
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			mu.Lock()
			allCalls++
			if campusID == "04" {
				shaheCalls++
			}
			mu.Unlock()
			if campusID == "04" {
				return nil, newJWError(jwErrorQuery, "jw query", nil, "shahe still down")
			}
			return []model.JWClassInfo{{
				NodeName:   "1",
				NodeTime:   "08:00-08:45",
				Classrooms: "教学实验综合楼-N101(10)",
			}}, nil
		},
	}
	fixed := time.Date(2026, 7, 9, 12, 0, 0, 0, businessLocation)
	clock := newFakeClock(fixed)
	svc := newTestServiceWithOptions(t, client, ClassroomServiceOptions{
		TokenOverride: "token",
		Clock:         clock,
	})

	seedCache(t, svc, &model.TodayClassrooms{
		Date:       fixed.Format("2006-01-02"),
		UpdatedAt:  fixed.Add(-time.Minute),
		ExpiresAt:  fixed.Add(4 * time.Minute), // still inside full fresh TTL
		StaleUntil: endOfDay(fixed),
		Campuses: []model.CampusInfo{
			{ID: "01", Name: "西土城", Buildings: []model.BuildingInfo{{Name: "教学实验综合楼"}}},
			{ID: "04", Name: "沙河"},
		},
		Error: &model.APIError{
			Type:    string(jwErrorQuery),
			Message: partialCampusErrorMessage,
		},
	})

	resp, err := svc.GetTodayClassrooms(context.Background())
	if err != nil {
		t.Fatalf("GetTodayClassrooms() error = %v", err)
	}
	if resp.Stale {
		t.Fatal("expected immediate partial response to stay non-stale")
	}
	if resp.Error == nil || resp.Error.Message != partialCampusErrorMessage {
		t.Fatalf("expected partial error preserved, got %#v", resp.Error)
	}

	waitFor(t, time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return shaheCalls >= 1
	})
	mu.Lock()
	firstShahe := shaheCalls
	firstAll := allCalls
	mu.Unlock()
	if firstShahe < 1 {
		t.Fatalf("expected failed campus to be re-queried inside fresh TTL, shaheCalls=%d", firstShahe)
	}

	// Same clock (still inside ExpiresAt and inside partial backoff): must not thrash JW.
	resp2, err := svc.GetTodayClassrooms(context.Background())
	if err != nil {
		t.Fatalf("GetTodayClassrooms() second error = %v", err)
	}
	if resp2.Error == nil {
		t.Fatalf("expected partial error on backoff serve, got %#v", resp2)
	}
	// Give any accidental second refresh a moment to start.
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	secondShahe := shaheCalls
	secondAll := allCalls
	mu.Unlock()
	if secondShahe != firstShahe || secondAll != firstAll {
		t.Fatalf("expected partial backoff to suppress re-query, calls before=%d/%d after=%d/%d",
			firstAll, firstShahe, secondAll, secondShahe)
	}
}

func TestGetTodayClassroomsPartialErrorRefreshCanRecoverFailedCampus(t *testing.T) {
	var shaheCalls atomic.Int32
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			if campusID == "04" {
				shaheCalls.Add(1)
				return []model.JWClassInfo{{
					NodeName:   "1",
					NodeTime:   "08:00-08:45",
					Classrooms: "沙河教学楼-S101(40)",
				}}, nil
			}
			return []model.JWClassInfo{{
				NodeName:   "1",
				NodeTime:   "08:00-08:45",
				Classrooms: "教学实验综合楼-N101(10)",
			}}, nil
		},
	}
	fixed := time.Date(2026, 7, 9, 12, 0, 0, 0, businessLocation)
	clock := newFakeClock(fixed)
	svc := newTestServiceWithOptions(t, client, ClassroomServiceOptions{
		TokenOverride: "token",
		Clock:         clock,
	})

	seedCache(t, svc, &model.TodayClassrooms{
		Date:       fixed.Format("2006-01-02"),
		UpdatedAt:  fixed.Add(-time.Minute),
		ExpiresAt:  fixed.Add(classroomFreshTTL),
		StaleUntil: endOfDay(fixed),
		Campuses: []model.CampusInfo{
			{ID: "01", Name: "西土城"},
			{ID: "04", Name: "沙河"},
		},
		Error: &model.APIError{
			Type:    string(jwErrorQuery),
			Message: partialCampusErrorMessage,
		},
	})

	resp, err := svc.GetTodayClassrooms(context.Background())
	if err != nil {
		t.Fatalf("GetTodayClassrooms() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected first response to still carry partial error while refresh runs")
	}

	waitFor(t, time.Second, func() bool {
		cached, ok := svc.getCachedTodayClassrooms()
		return ok && cached.Error == nil && shaheCalls.Load() >= 1
	})
	cached, ok := svc.getCachedTodayClassrooms()
	if !ok || cached.Error != nil {
		t.Fatalf("expected recovered cache without partial error, ok=%t cached=%#v", ok, cached)
	}
	shahe := requireCampusByID(t, cached.Campuses, "04")
	if len(shahe.Buildings) == 0 {
		t.Fatalf("expected recovered shahe buildings, got %#v", shahe)
	}
}
