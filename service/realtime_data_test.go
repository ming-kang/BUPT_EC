package service

import (
	"BUPT_EC/config"
	"BUPT_EC/logs"
	"BUPT_EC/service/model"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

func init() {
	logs.Init(false)
	config.InitConfig()
}

type mockJWClient struct {
	queryCampus func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error)
	login       func(ctx context.Context, apiURL string) (string, error)
	fetchAPIURL func(ctx context.Context) (string, error)
}

func (m *mockJWClient) QueryCampus(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
	if m.queryCampus == nil {
		return nil, errors.New("mockJWClient.QueryCampus is not configured")
	}
	return m.queryCampus(ctx, apiURL, campusID, token)
}

func (m *mockJWClient) Login(ctx context.Context, apiURL string) (string, error) {
	if m.login == nil {
		return "mock-token", nil
	}
	return m.login(ctx, apiURL)
}

func (m *mockJWClient) FetchAPIURL(ctx context.Context) (string, error) {
	if m.fetchAPIURL == nil {
		return DefaultAPIURL, nil
	}
	return m.fetchAPIURL(ctx)
}

func newTestService(t *testing.T, client JWClient) *ClassroomService {
	t.Helper()
	svc := newClassroomService(config.GetConfig(), gocache.New(5*time.Minute, time.Minute), client)
	t.Cleanup(svc.refreshWorkers.Wait)
	return svc
}

func requireJWCredentials(t *testing.T) {
	t.Helper()
	if os.Getenv(LoginTokenKey) != "" {
		return
	}
	if os.Getenv(LoginUsernameKey) == "" || os.Getenv(LoginPasswordKey) == "" {
		t.Skip("JW_TOKEN or JW_USERNAME/JW_PASSWORD are required for integration login/query tests")
	}
}

func TestEncryptJWPassword(t *testing.T) {
	encrypted, err := encryptJWPassword("test-password")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "" {
		t.Fatal("encrypted password should not be empty")
	}
	if encrypted == "test-password" {
		t.Fatal("encrypted password should not equal plaintext")
	}
}

func TestParseRoom(t *testing.T) {
	building, room, displayName, capacity, ok := parseRoom("教学实验综合楼-N104(229)")
	if !ok {
		t.Fatal("expected room to parse")
	}
	if building != "教学实验综合楼" || room != "N104" || displayName != "教学实验综合楼-N104" || capacity != 229 {
		t.Fatalf("unexpected parsed room: %q %q %q %d", building, room, displayName, capacity)
	}

	building, room, displayName, capacity, ok = parseRoom("未来学习大楼-202-203(60)")
	if !ok {
		t.Fatal("expected merged room to parse")
	}
	if building != "未来学习大楼" || room != "202-203" || displayName != "未来学习大楼-202-203" || capacity != 60 {
		t.Fatalf("unexpected parsed merged room: %q %q %q %d", building, room, displayName, capacity)
	}

	building, room, displayName, capacity, ok = parseRoom("教学实验综合楼-N104（229）")
	if !ok {
		t.Fatal("expected full-width parentheses room to parse")
	}
	if building != "教学实验综合楼" || room != "N104" || displayName != "教学实验综合楼-N104" || capacity != 229 {
		t.Fatalf("unexpected parsed full-width room: %q %q %q %d", building, room, displayName, capacity)
	}
}

func TestBuildCampusInfoDeduplicatesRooms(t *testing.T) {
	campus := buildCampusInfo(config.CampusConfig{ID: "01", Name: "西土城"}, []model.JWClassInfo{
		{
			NodeName:   "1",
			NodeTime:   "08:00-08:45",
			Classrooms: "教学实验综合楼-N104(229),教学实验综合楼-N104(229)",
		},
	})

	if len(campus.Nodes) != 1 {
		t.Fatalf("expected one node, got %d", len(campus.Nodes))
	}
	if campus.Nodes[0].RoomCount != 1 {
		t.Fatalf("expected deduplicated room count 1, got %d", campus.Nodes[0].RoomCount)
	}
	if len(campus.Buildings) != 1 || len(campus.Buildings[0].Rooms) != 1 {
		t.Fatalf("expected one deduplicated room, got %#v", campus.Buildings)
	}
	room := campus.Buildings[0].Rooms[0]
	if len(room.FreeNodes) != 1 || room.FreeNodes[0] != 1 {
		t.Fatalf("expected one deduplicated free node, got %#v", room.FreeNodes)
	}
	if len(room.FreeTimes) != 1 || room.FreeTimes[0].Node != 1 {
		t.Fatalf("expected one deduplicated free time, got %#v", room.FreeTimes)
	}
}

func TestValidateJWAPIURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "default host appends slash",
			input: "https://jwglweixin.bupt.edu.cn/bjyddx",
			want:  "https://jwglweixin.bupt.edu.cn/bjyddx/",
		},
		{
			name:  "allowed bupt subdomain strips query and fragment",
			input: "https://api.bupt.edu.cn/base?token=secret#fragment",
			want:  "https://api.bupt.edu.cn/base/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateJWAPIURL(tt.input)
			if err != nil {
				t.Fatalf("validateJWAPIURL() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("validateJWAPIURL() = %q, want %q", got, tt.want)
			}
		})
	}

	invalid := []string{
		"http://jwglweixin.bupt.edu.cn/bjyddx/",
		"https://evil.example/bjyddx/",
		"https://evilbupt.edu.cn/bjyddx/",
		"https://user@jwglweixin.bupt.edu.cn/bjyddx/",
		"https://jwglweixin.bupt.edu.cn:8443/bjyddx/",
	}
	for _, input := range invalid {
		t.Run(input, func(t *testing.T) {
			if got, err := validateJWAPIURL(input); err == nil {
				t.Fatalf("validateJWAPIURL() = %q, want error", got)
			}
		})
	}
}

func TestEnsureTokenUsesOverrideOnlyWithoutForceRefresh(t *testing.T) {
	svc := newTestService(t, &defaultJWClient{})
	t.Setenv(LoginTokenKey, "override-token")
	t.Setenv(LoginUsernameKey, "")
	t.Setenv(LoginPasswordKey, "")

	token, err := svc.tokenManager.EnsureToken(context.Background(), false)
	if err != nil {
		t.Fatalf("EnsureToken(false) error = %v", err)
	}
	if token != "override-token" {
		t.Fatalf("EnsureToken(false) = %q, want override-token", token)
	}

	svc.tokenManager.setAPIURL(DefaultAPIURL)
	token, err = svc.tokenManager.EnsureToken(context.Background(), true)
	if err == nil {
		t.Fatal("EnsureToken(true) expected configuration error")
	}
	if token == "override-token" {
		t.Fatal("EnsureToken(true) unexpectedly returned JW_TOKEN override")
	}
	if !isJWErrorKind(err, jwErrorConfig) {
		t.Fatalf("EnsureToken(true) error kind = %s, want %s", classifyError(err), jwErrorConfig)
	}
}

func TestGetCachedTodayClassroomsRejectsCrossDayCache(t *testing.T) {
	svc := newTestService(t, &mockJWClient{})
	yesterday := time.Now().Add(-24 * time.Hour)
	svc.cache.Set(TodayCacheKey, &model.TodayClassrooms{
		Date:       yesterday.Format("2006-01-02"),
		ExpiresAt:  yesterday.Add(time.Hour),
		StaleUntil: yesterday.Add(time.Hour),
	}, time.Minute)

	if cached, ok := svc.getCachedTodayClassrooms(); ok {
		t.Fatalf("expected cross-day cache miss, got %#v", cached)
	}

	now := time.Now()
	svc.cache.Set(TodayCacheKey, &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		ExpiresAt:  now.Add(time.Hour),
		StaleUntil: endOfDay(now),
	}, time.Minute)

	if cached, ok := svc.getCachedTodayClassrooms(); !ok || cached.Date != now.Format("2006-01-02") {
		t.Fatalf("expected same-day cache hit, got %#v ok=%t", cached, ok)
	}
}

func TestCacheExpirationAlwaysPositive(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, businessLocation)
	staleUntil := endOfDay(now)
	d := cacheExpiration(now, staleUntil)
	if d <= 0 {
		t.Fatalf("cacheExpiration positive day = %s, want > 0", d)
	}
	if want := staleUntil.Sub(now); d != want {
		t.Fatalf("cacheExpiration = %s, want %s", d, want)
	}

	// Past or equal StaleUntil must not yield a non-positive go-cache TTL.
	if got := cacheExpiration(now, now); got != time.Second {
		t.Fatalf("cacheExpiration(now, now) = %s, want 1s", got)
	}
	if got := cacheExpiration(now, now.Add(-time.Hour)); got != time.Second {
		t.Fatalf("cacheExpiration past StaleUntil = %s, want 1s", got)
	}
	if got := cacheExpiration(now, now.Add(500*time.Millisecond)); got != time.Second {
		t.Fatalf("cacheExpiration sub-second remaining = %s, want 1s", got)
	}
}

func TestDoRefreshStampsCacheAtCompletionAcrossMidnight(t *testing.T) {
	beforeMidnight := time.Date(2026, 7, 9, 23, 59, 50, 0, businessLocation)
	afterMidnight := time.Date(2026, 7, 10, 0, 0, 5, 0, businessLocation)

	var mu sync.Mutex
	current := beforeMidnight
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			mu.Lock()
			current = afterMidnight
			mu.Unlock()
			return []model.JWClassInfo{{
				NodeName:   "1",
				NodeTime:   "08:00-08:45",
				Classrooms: fmt.Sprintf("教学实验综合楼-%s(10)", campusID),
			}}, nil
		},
	}
	svc := newTestService(t, client)
	svc.now = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return current
	}

	resp, err := svc.doRefreshTodayClassrooms(context.Background())
	if err != nil {
		t.Fatalf("doRefreshTodayClassrooms() error = %v", err)
	}

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
	if d := cacheExpiration(afterMidnight, resp.StaleUntil); d <= 0 {
		t.Fatalf("cache TTL would be non-positive: %s", d)
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

func TestGetTodayClassroomsReturnsFreshCacheWithoutJWQuery(t *testing.T) {
	var queryCalls atomic.Int32
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			queryCalls.Add(1)
			return nil, errors.New("fresh cache should not query JW")
		},
	}
	classroomServiceUnderTest := newTestService(t, client)

	now := time.Now()
	classroomServiceUnderTest.cache.Set(TodayCacheKey, &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		UpdatedAt:  now,
		ExpiresAt:  now.Add(time.Minute),
		StaleUntil: endOfDay(now),
		Campuses: []model.CampusInfo{
			{ID: "cached", Name: "cached campus"},
		},
	}, time.Hour)

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
	t.Setenv(LoginTokenKey, "fixture-token")
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
	classroomServiceUnderTest := newTestService(t, client)

	response, err := classroomServiceUnderTest.QueryAll(context.Background())
	if err != nil {
		t.Fatalf("QueryAll() error = %v", err)
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

func requireCampusByID(t *testing.T, campuses []model.CampusInfo, campusID string) model.CampusInfo {
	t.Helper()
	for _, campus := range campuses {
		if campus.ID == campusID {
			return campus
		}
	}
	t.Fatalf("campus %q not found in %#v", campusID, campuses)
	return model.CampusInfo{}
}

func requireBuildingByName(t *testing.T, buildings []model.BuildingInfo, buildingName string) model.BuildingInfo {
	t.Helper()
	for _, building := range buildings {
		if building.Name == buildingName {
			return building
		}
	}
	t.Fatalf("building %q not found in %#v", buildingName, buildings)
	return model.BuildingInfo{}
}

func requireRoomByDisplayName(t *testing.T, rooms []model.RoomInfo, displayName string) model.RoomInfo {
	t.Helper()
	for _, room := range rooms {
		if room.DisplayName == displayName {
			return room
		}
	}
	t.Fatalf("room %q not found in %#v", displayName, rooms)
	return model.RoomInfo{}
}

func TestClassifyErrorHandlesJoinedJWError(t *testing.T) {
	err := errors.Join(context.DeadlineExceeded, newJWError(jwErrorAuth, "jw query", nil, "token expired"))
	if got := classifyError(err); got != string(jwErrorAuth) {
		t.Fatalf("classifyError() = %q, want %q", got, jwErrorAuth)
	}
}

func TestParseJWQueryResponseClassifiesBusinessAuthCode(t *testing.T) {
	_, err := parseJWQueryResponse([]byte(`{"code":"401","Msg":"illegal access","data":""}`))
	if err == nil {
		t.Fatal("expected auth error")
	}
	if !isJWErrorKind(err, jwErrorAuth) {
		t.Fatalf("parseJWQueryResponse() error kind = %s, want %s", classifyError(err), jwErrorAuth)
	}
}

func TestClassifyJWHTTPErrorUsesBusinessAuthCode(t *testing.T) {
	kind, message := classifyJWHTTPError(http.StatusInternalServerError, []byte(`{"code":"401","Msg":"illegal access","data":""}`))
	if kind != jwErrorAuth {
		t.Fatalf("classifyJWHTTPError() kind = %s, want %s", kind, jwErrorAuth)
	}
	if message != "illegal access" {
		t.Fatalf("classifyJWHTTPError() message = %q, want illegal access", message)
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

	now := time.Now()
	svc.cache.Set(TodayCacheKey, &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		UpdatedAt:  now.Add(-time.Hour),
		ExpiresAt:  now.Add(-time.Minute),
		StaleUntil: endOfDay(now),
		Campuses: []model.CampusInfo{
			{ID: "cached", Name: "cached"},
		},
	}, time.Hour)

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

	svc.StartWarmup()
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

	now := time.Now()
	svc.cache.Set(TodayCacheKey, &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		UpdatedAt:  now.Add(-time.Hour),
		ExpiresAt:  now.Add(-time.Minute),
		StaleUntil: endOfDay(now),
		Campuses:   []model.CampusInfo{{ID: "cached", Name: "cached"}},
	}, time.Hour)

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
	resp, err := svc.QueryAll(context.Background())
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("QueryAll() error = %v", err)
	}
	if len(resp.Campuses) != 2 {
		t.Fatalf("expected two campuses, got %d", len(resp.Campuses))
	}
	if elapsed > 550*time.Millisecond {
		t.Fatalf("expected concurrent campus queries, took %s", elapsed)
	}
}

func TestQueryCampusRefreshesTokenAfterAuthFailure(t *testing.T) {
	t.Setenv(LoginTokenKey, "")
	var refreshCalls atomic.Int32
	var seenTokens []string
	client := &mockJWClient{
		login: func(ctx context.Context, apiURL string) (string, error) {
			refreshCalls.Add(1)
			return "new-token", nil
		},
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			seenTokens = append(seenTokens, token)
			if token == "old-token" {
				return nil, newJWError(jwErrorAuth, "jw query", nil, "token expired")
			}
			return []model.JWClassInfo{{NodeName: "1", NodeTime: "08:00-08:45", Classrooms: "教学实验综合楼-N101(10)"}}, nil
		},
	}
	svc := newTestService(t, client)
	svc.tokenManager.setToken("old-token")

	rows, err := svc.queryCampus(context.Background(), "01", false)
	if err != nil {
		t.Fatalf("queryCampus() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected rows after token refresh, got %#v", rows)
	}
	if refreshCalls.Load() != 1 {
		t.Fatalf("expected one token refresh, got %d", refreshCalls.Load())
	}
	if len(seenTokens) != 2 || seenTokens[0] != "old-token" || seenTokens[1] != "new-token" {
		t.Fatalf("unexpected token sequence: %#v", seenTokens)
	}
}

func waitFor(t *testing.T, timeout time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}

func TestEnsureTokenDoesNotReapplyInvalidatedJWToken(t *testing.T) {
	t.Setenv(LoginTokenKey, "bad-override")
	t.Setenv(LoginUsernameKey, "user")
	t.Setenv(LoginPasswordKey, "pass")

	var loginCalls atomic.Int32
	client := &mockJWClient{
		login: func(ctx context.Context, apiURL string) (string, error) {
			loginCalls.Add(1)
			return "fresh-login-token", nil
		},
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			if token == "bad-override" {
				return nil, newJWError(jwErrorAuth, "jw query", nil, "token invalid")
			}
			if token != "fresh-login-token" {
				return nil, fmt.Errorf("unexpected token %q", token)
			}
			return []model.JWClassInfo{{
				NodeName:   "1",
				NodeTime:   "08:00-08:45",
				Classrooms: "教学实验综合楼-N101(10)",
			}}, nil
		},
	}
	svc := newTestService(t, client)

	rows, err := svc.queryCampus(context.Background(), "01", false)
	if err != nil {
		t.Fatalf("queryCampus() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected rows after override invalidation, got %#v", rows)
	}
	if loginCalls.Load() != 1 {
		t.Fatalf("loginCalls = %d, want 1", loginCalls.Load())
	}

	token, err := svc.tokenManager.EnsureToken(context.Background(), false)
	if err != nil {
		t.Fatalf("EnsureToken(false) after recovery error = %v", err)
	}
	if token != "fresh-login-token" {
		t.Fatalf("EnsureToken(false) = %q, want fresh-login-token (must not reapply JW_TOKEN)", token)
	}
	if loginCalls.Load() != 1 {
		t.Fatalf("loginCalls after EnsureToken = %d, want 1", loginCalls.Load())
	}
}

func TestDoRefreshPartialCampusSuccess(t *testing.T) {
	t.Setenv(LoginTokenKey, "token")
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			if campusID == "04" {
				return nil, newJWError(jwErrorQuery, "jw query", nil, "shahe down")
			}
			return []model.JWClassInfo{{
				NodeName:   "1",
				NodeTime:   "08:00-08:45",
				Classrooms: "教学实验综合楼-N101(10)",
			}}, nil
		},
	}
	svc := newTestService(t, client)

	resp, err := svc.QueryAll(context.Background())
	if err != nil {
		t.Fatalf("QueryAll() error = %v", err)
	}
	if resp.Error == nil || resp.Error.Message != partialCampusErrorMessage {
		t.Fatalf("expected partial campus error, got %#v", resp.Error)
	}
	xitucheng := requireCampusByID(t, resp.Campuses, "01")
	if len(xitucheng.Buildings) == 0 {
		t.Fatal("expected successful campus to have buildings")
	}
	shahe := requireCampusByID(t, resp.Campuses, "04")
	if len(shahe.Buildings) != 0 {
		t.Fatalf("expected empty skeleton for failed campus without prior cache, got %#v", shahe)
	}

	cached, ok := svc.getCachedTodayClassrooms()
	if !ok || cached.Error == nil {
		t.Fatalf("expected partial result cached, ok=%t cached=%#v", ok, cached)
	}
}

func TestDoRefreshPartialCampusMergesPreviousCache(t *testing.T) {
	t.Setenv(LoginTokenKey, "token")
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			if campusID == "04" {
				return nil, newJWError(jwErrorQuery, "jw query", nil, "shahe down")
			}
			return []model.JWClassInfo{{
				NodeName:   "1",
				NodeTime:   "08:00-08:45",
				Classrooms: "教学实验综合楼-N101(10)",
			}}, nil
		},
	}
	svc := newTestService(t, client)
	now := svc.now()
	svc.cache.Set(TodayCacheKey, &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		ExpiresAt:  now.Add(time.Minute),
		StaleUntil: endOfDay(now),
		Campuses: []model.CampusInfo{
			{ID: "01", Name: "西土城"},
			{
				ID:   "04",
				Name: "沙河",
				Buildings: []model.BuildingInfo{
					{Name: "旧楼", Rooms: []model.RoomInfo{{Name: "1", DisplayName: "旧楼-1"}}},
				},
			},
		},
	}, time.Hour)

	resp, err := svc.QueryAll(context.Background())
	if err != nil {
		t.Fatalf("QueryAll() error = %v", err)
	}
	shahe := requireCampusByID(t, resp.Campuses, "04")
	if len(shahe.Buildings) != 1 || shahe.Buildings[0].Name != "旧楼" {
		t.Fatalf("expected previous shahe campus data merged, got %#v", shahe)
	}
}

func TestDoRefreshAllCampusesFail(t *testing.T) {
	t.Setenv(LoginTokenKey, "token")
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			return nil, newJWError(jwErrorQuery, "jw query", nil, "down")
		},
	}
	svc := newTestService(t, client)

	_, err := svc.QueryAll(context.Background())
	if err == nil {
		t.Fatal("QueryAll() expected error when all campuses fail")
	}
	if _, ok := svc.getCachedTodayClassrooms(); ok {
		t.Fatal("expected no cache update when all campuses fail")
	}
}

// Partial-campus error must not freeze retries for the full fresh TTL: a later
// Get inside the old ExpiresAt window should still kick a background refresh
// (subject to single-flight + 30s backoff).
func TestGetTodayClassroomsRetriesPartialErrorWithinFreshTTL(t *testing.T) {
	t.Setenv(LoginTokenKey, "token")

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
	svc := newTestService(t, client)

	fixed := time.Date(2026, 7, 9, 12, 0, 0, 0, businessLocation)
	svc.now = func() time.Time { return fixed }

	svc.cache.Set(TodayCacheKey, &model.TodayClassrooms{
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
	}, time.Hour)

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
	t.Setenv(LoginTokenKey, "token")

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
	svc := newTestService(t, client)

	fixed := time.Date(2026, 7, 9, 12, 0, 0, 0, businessLocation)
	svc.now = func() time.Time { return fixed }

	svc.cache.Set(TodayCacheKey, &model.TodayClassrooms{
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
	}, time.Hour)

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

func TestRuntimeStatusCacheStaleOnlyWhenPastFreshTTL(t *testing.T) {
	svc := newTestService(t, &mockJWClient{})
	fixed := time.Date(2026, 7, 9, 12, 0, 0, 0, businessLocation)
	svc.now = func() time.Time { return fixed }

	svc.cache.Set(TodayCacheKey, &model.TodayClassrooms{
		Date:       fixed.Format("2006-01-02"),
		ExpiresAt:  fixed.Add(time.Minute),
		StaleUntil: endOfDay(fixed),
	}, time.Hour)

	status := svc.GetRuntimeStatus()
	if !status.CacheAvailable || !status.CacheFresh || status.CacheStale {
		t.Fatalf("fresh cache status = %#v, want fresh and not stale", status)
	}

	svc.cache.Set(TodayCacheKey, &model.TodayClassrooms{
		Date:       fixed.Format("2006-01-02"),
		ExpiresAt:  fixed.Add(-time.Minute),
		StaleUntil: endOfDay(fixed),
	}, time.Hour)

	status = svc.GetRuntimeStatus()
	if !status.CacheAvailable || status.CacheFresh || !status.CacheStale {
		t.Fatalf("expired-but-usable cache status = %#v, want stale", status)
	}
}

func TestIsAuthFailureMessageDoesNotMatchBareExpiry(t *testing.T) {
	if isAuthFailureMessage("活动已过期") {
		t.Fatal("bare 过期 should not be treated as auth failure")
	}
	if isAuthFailureMessage("数据失效") {
		t.Fatal("bare 失效 should not be treated as auth failure")
	}
	if !isAuthFailureMessage("token 已过期") {
		t.Fatal("token-related message should be auth failure")
	}
	if !isAuthFailureMessage("请重新登录") {
		t.Fatal("重新登录 should be auth failure")
	}
}

func TestSafeErrorMessageForNoTodayCache(t *testing.T) {
	msg := SafeErrorMessage(ErrNoTodayCache)
	if msg != "暂无可用的今日空教室数据，请稍后重试" {
		t.Fatalf("SafeErrorMessage(ErrNoTodayCache) = %q", msg)
	}
}

func TestLogin(t *testing.T) {
	requireJWCredentials(t)
	svc := newTestService(t, &defaultJWClient{})
	err := svc.Login(context.Background())
	if err != nil {
		t.Error(err)
	}
}

func TestQueryOne(t *testing.T) {
	requireJWCredentials(t)
	svc := newTestService(t, &defaultJWClient{})
	err := svc.Login(context.Background())
	if err != nil {
		t.Error(err)
	}
	rows, err := svc.QueryOne(context.Background(), "01")
	if err != nil {
		t.Error(err)
	}
	if len(rows) == 0 {
		t.Error("expected classroom rows")
	}
}

func TestQueryAll(t *testing.T) {
	requireJWCredentials(t)
	svc := newTestService(t, &defaultJWClient{})
	err := svc.Login(context.Background())
	if err != nil {
		t.Error(err)
	}
	ans, err := svc.QueryAll(context.Background())
	if err != nil {
		t.Error(err)
	}
	if ans == nil {
		t.Fatal("expected response")
	}
	if len(ans.Campuses) != 2 {
		t.Fatalf("expected 2 campuses, got %d", len(ans.Campuses))
	}
}
