package service

import (
	"BUPT_EC/cache"
	"BUPT_EC/config"
	"BUPT_EC/logs"
	"BUPT_EC/service/model"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func init() {
	logs.Init(false)
	config.InitConfig()
	cache.InitCache()
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
	ResetRuntimeStateForTest()
	t.Setenv(LoginTokenKey, "override-token")
	t.Setenv(LoginUsernameKey, "")
	t.Setenv(LoginPasswordKey, "")

	token, err := tokenManager.EnsureToken(context.Background(), false)
	if err != nil {
		t.Fatalf("EnsureToken(false) error = %v", err)
	}
	if token != "override-token" {
		t.Fatalf("EnsureToken(false) = %q, want override-token", token)
	}

	tokenManager.setAPIURL(DefaultAPIURL)
	token, err = tokenManager.EnsureToken(context.Background(), true)
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
	ResetRuntimeStateForTest()
	yesterday := time.Now().Add(-24 * time.Hour)
	cache.SetCache(TodayCacheKey, &model.TodayClassrooms{
		Date:       yesterday.Format("2006-01-02"),
		ExpiresAt:  yesterday.Add(time.Hour),
		StaleUntil: yesterday.Add(time.Hour),
	}, time.Minute)

	if cached, ok := getCachedTodayClassrooms(); ok {
		t.Fatalf("expected cross-day cache miss, got %#v", cached)
	}

	now := time.Now()
	cache.SetCache(TodayCacheKey, &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		ExpiresAt:  now.Add(time.Hour),
		StaleUntil: endOfDay(now),
	}, time.Minute)

	if cached, ok := getCachedTodayClassrooms(); !ok || cached.Date != now.Format("2006-01-02") {
		t.Fatalf("expected same-day cache hit, got %#v ok=%t", cached, ok)
	}
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
	ResetRuntimeStateForTest()
	now := time.Now()
	cache.SetCache(TodayCacheKey, &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		UpdatedAt:  now.Add(-time.Hour),
		ExpiresAt:  now.Add(-time.Minute),
		StaleUntil: endOfDay(now),
		Campuses: []model.CampusInfo{
			{ID: "cached", Name: "cached"},
		},
	}, time.Hour)

	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	var calls atomic.Int32
	setQueryCampusForRefresh(t, func(ctx context.Context, campusID string, forceRefresh bool) ([]model.JWClassInfo, error) {
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
	})

	start := time.Now()
	resp, err := GetTodayClassrooms(context.Background())
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
		cached, ok := getCachedTodayClassrooms()
		return ok && len(cached.Campuses) == 2
	})
	if calls.Load() != 2 {
		t.Fatalf("expected one query per campus, got %d", calls.Load())
	}
}

func TestGetTodayClassroomsBroadcastsRefreshResultToConcurrentWaiters(t *testing.T) {
	ResetRuntimeStateForTest()

	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	setQueryCampusForRefresh(t, func(ctx context.Context, campusID string, forceRefresh bool) ([]model.JWClassInfo, error) {
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
	})

	const waiters = 8
	errs := make(chan error, waiters)
	var wg sync.WaitGroup
	wg.Add(waiters)
	for i := 0; i < waiters; i++ {
		go func() {
			defer wg.Done()
			resp, err := GetTodayClassrooms(context.Background())
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
	ResetRuntimeStateForTest()

	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	setQueryCampusForRefresh(t, func(ctx context.Context, campusID string, forceRefresh bool) ([]model.JWClassInfo, error) {
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
	})

	StartClassroomWarmup()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected warmup refresh to start")
	}

	errCh := make(chan error, 1)
	go func() {
		resp, err := GetTodayClassrooms(context.Background())
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
	ResetRuntimeStateForTest()
	now := time.Now()
	cache.SetCache(TodayCacheKey, &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		UpdatedAt:  now.Add(-time.Hour),
		ExpiresAt:  now.Add(-time.Minute),
		StaleUntil: endOfDay(now),
		Campuses:   []model.CampusInfo{{ID: "cached", Name: "cached"}},
	}, time.Hour)

	var calls atomic.Int32
	setQueryCampusForRefresh(t, func(ctx context.Context, campusID string, forceRefresh bool) ([]model.JWClassInfo, error) {
		calls.Add(1)
		return nil, newJWError(jwErrorQuery, "jw query", nil, "upstream failed")
	})

	resp, err := GetTodayClassrooms(context.Background())
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

	resp, err = GetTodayClassrooms(context.Background())
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
	ResetRuntimeStateForTest()
	setQueryCampusForRefresh(t, func(ctx context.Context, campusID string, forceRefresh bool) ([]model.JWClassInfo, error) {
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
	})

	start := time.Now()
	resp, err := QueryAll(context.Background())
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
	ResetRuntimeStateForTest()
	tokenManager.setToken("old-token")
	var refreshCalls atomic.Int32
	setRefreshTokenFunc(t, func(m *TokenManager, ctx context.Context) (string, error) {
		refreshCalls.Add(1)
		return "new-token", nil
	})

	var seenTokens []string
	setQueryCampusWithTokenFunc(t, func(ctx context.Context, campusID string, token string) ([]model.JWClassInfo, error) {
		seenTokens = append(seenTokens, token)
		if token == "old-token" {
			return nil, newJWError(jwErrorAuth, "jw query", nil, "token expired")
		}
		return []model.JWClassInfo{{NodeName: "1", NodeTime: "08:00-08:45", Classrooms: "教学实验综合楼-N101(10)"}}, nil
	})

	rows, err := queryCampus(context.Background(), "01", false)
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

func setQueryCampusForRefresh(t *testing.T, fn func(context.Context, string, bool) ([]model.JWClassInfo, error)) {
	t.Helper()
	old := queryCampusForRefresh
	queryCampusForRefresh = fn
	t.Cleanup(func() {
		queryCampusForRefresh = old
	})
}

func setQueryCampusWithTokenFunc(t *testing.T, fn func(context.Context, string, string) ([]model.JWClassInfo, error)) {
	t.Helper()
	old := queryCampusWithTokenFunc
	queryCampusWithTokenFunc = fn
	t.Cleanup(func() {
		queryCampusWithTokenFunc = old
	})
}

func setRefreshTokenFunc(t *testing.T, fn func(*TokenManager, context.Context) (string, error)) {
	t.Helper()
	old := refreshTokenFunc
	refreshTokenFunc = fn
	t.Cleanup(func() {
		refreshTokenFunc = old
	})
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
	ResetRuntimeStateForTest()
	err := Login(context.Background())
	if err != nil {
		t.Error(err)
	}
}

func TestQueryOne(t *testing.T) {
	requireJWCredentials(t)
	ResetRuntimeStateForTest()
	err := Login(context.Background())
	if err != nil {
		t.Error(err)
	}
	rows, err := QueryOne(context.Background(), "01")
	if err != nil {
		t.Error(err)
	}
	if len(rows) == 0 {
		t.Error("expected classroom rows")
	}
}

func TestQueryAll(t *testing.T) {
	requireJWCredentials(t)
	ResetRuntimeStateForTest()
	err := Login(context.Background())
	if err != nil {
		t.Error(err)
	}
	ans, err := QueryAll(context.Background())
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
