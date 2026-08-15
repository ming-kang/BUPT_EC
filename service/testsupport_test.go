package service

// testsupport_test.go centralizes the shared white-box fixtures for the
// service package: the mock JW client, test service constructors, and
// assertion helpers. Integration-only helpers (credential skips, integration
// service assembly) live in integration_test.go behind the "integration"
// build tag; newHTTPJWClientForTest stays here because offline unit tests use
// it too.

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"BUPT_EC/config"
	"BUPT_EC/logs"
	"BUPT_EC/service/model"
	"BUPT_EC/utils"
)

func init() {
	if err := logs.Init(false, false); err != nil {
		panic(err)
	}
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
	return newTestServiceWithOverride(t, client, "")
}

func newTestServiceWithOverride(t *testing.T, client JWClient, tokenOverride string) *ClassroomService {
	t.Helper()
	return newTestServiceWithOptions(t, client, ClassroomServiceOptions{
		TokenOverride: tokenOverride,
	})
}

// newTestServiceWithOptions builds an isolated ClassroomService for unit tests.
// Nil Clock keeps the real wall clock; nil BackoffRandom defaults to sample 0.5
// so total-failure deadlines stay deterministic unless a test injects its own.
func newTestServiceWithOptions(t *testing.T, client JWClient, options ClassroomServiceOptions) *ClassroomService {
	t.Helper()
	if len(options.Campuses) == 0 {
		options.Campuses = []config.CampusConfig{
			{ID: "01", Name: "西土城"},
			{ID: "04", Name: "沙河"},
		}
	}
	if options.BackoffRandom == nil {
		options.BackoffRandom = func() float64 { return 0.5 }
	}
	svc, err := NewClassroomService(options, client)
	if err != nil {
		t.Fatalf("NewClassroomService() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := svc.Shutdown(ctx); err != nil {
			t.Errorf("Shutdown() cleanup error = %v", err)
		}
	})
	return svc
}

func newHTTPJWClientForTest(t *testing.T, username, password string) JWClient {
	t.Helper()
	client, err := NewJWClient(username, password, utils.NewHTTPClient())
	if err != nil {
		t.Fatalf("NewJWClient() error = %v", err)
	}
	return client
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

func tokenTestRows(campusID string) []model.JWClassInfo {
	return []model.JWClassInfo{{
		NodeName:   "1",
		NodeTime:   "08:00-08:45",
		Classrooms: fmt.Sprintf("教学实验综合楼-%s(10)", campusID),
	}}
}
