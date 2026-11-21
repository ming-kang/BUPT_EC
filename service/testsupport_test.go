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
	"sync"
	"testing"
	"time"

	"BUPT_EC/logs"
	"BUPT_EC/service/model"
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

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

var testNoon = time.Date(2026, 9, 1, 12, 0, 0, 0, businessLocation)

func newTestService(t *testing.T, client JWClient) (*ClassroomService, *fakeClock) {
	t.Helper()
	clock := &fakeClock{now: testNoon}
	svc := NewClassroomService(client)
	svc.clock = clock.Now
	return svc, clock
}

func startRun(t *testing.T, svc *ClassroomService) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		svc.Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
}

func newHTTPJWClientForTest(t *testing.T, username, password string) JWClient {
	t.Helper()
	client := NewJWClient(username, password, NewJWHTTPClient())
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
