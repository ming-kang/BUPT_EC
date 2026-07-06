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
