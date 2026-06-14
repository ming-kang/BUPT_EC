package service

import (
	"BUPT_EC/cache"
	"BUPT_EC/config"
	"BUPT_EC/logs"
	"BUPT_EC/service/model"
	"context"
	"errors"
	"os"
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
