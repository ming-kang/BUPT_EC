package service

// Partial-refresh outcome family: partial campus success, warning logging,
// full-refresh recovery, previous-cache merging, total failure, and
// latest-error precedence over stale partial warnings.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"BUPT_EC/service/model"
)

func TestDoRefreshPartialCampusSuccess(t *testing.T) {
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
	svc := newTestServiceWithOverride(t, client, "token")

	resp, err := svc.queryAll(context.Background())
	if err != nil {
		t.Fatalf("queryAll() error = %v", err)
	}
	if resp.Error == nil || resp.Error.Message != partialCampusErrorMessage {
		t.Fatalf("expected partial campus error, got %#v", resp.Error)
	}
	if !reflect.DeepEqual(resp.PartialCampuses, []string{"04"}) {
		t.Fatalf("partial_campuses = %#v, want [04]", resp.PartialCampuses)
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

	status := svc.GetRuntimeStatus()
	if !status.CacheAvailable || !status.CachePartial || !status.CacheFresh {
		t.Fatalf("partial runtime cache status = %#v", status)
	}
	if !reflect.DeepEqual(status.PartialCampuses, []string{"04"}) {
		t.Fatalf("runtime partial campuses = %#v, want [04]", status.PartialCampuses)
	}
	if status.LastRefreshWarning != partialCampusErrorMessage || status.LastRefreshError != "" {
		t.Fatalf("partial runtime refresh status = %#v", status)
	}
}

func TestPartialRefreshWritesWarningLog(t *testing.T) {
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

	previousLogger := slog.Default()
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	svc := newTestServiceWithOverride(t, client, "token")
	if _, err := svc.queryAll(context.Background()); err != nil {
		t.Fatalf("queryAll() error = %v", err)
	}
	logOutput := buf.String()
	if !strings.Contains(logOutput, "classroom refresh partially succeeded") ||
		!strings.Contains(logOutput, `"failed_campuses":["04"]`) {
		t.Fatalf("partial refresh warning log missing diagnostics: %s", logOutput)
	}
}

func TestFullRefreshClearsPartialRuntimeWarning(t *testing.T) {
	var failShahe atomic.Bool
	failShahe.Store(true)
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			if campusID == "04" && failShahe.Load() {
				return nil, newJWError(jwErrorQuery, "jw query", nil, "shahe down")
			}
			return []model.JWClassInfo{{
				NodeName:   "1",
				NodeTime:   "08:00-08:45",
				Classrooms: fmt.Sprintf("教学实验综合楼-%s(10)", campusID),
			}}, nil
		},
	}
	svc := newTestServiceWithOverride(t, client, "token")

	if _, err := svc.queryAll(context.Background()); err != nil {
		t.Fatalf("partial queryAll() error = %v", err)
	}
	failShahe.Store(false)
	resp, err := svc.queryAll(context.Background())
	if err != nil {
		t.Fatalf("full queryAll() error = %v", err)
	}
	if resp.Error != nil || len(resp.PartialCampuses) != 0 {
		t.Fatalf("full response retained partial state: %#v", resp)
	}
	status := svc.GetRuntimeStatus()
	if status.CachePartial || status.LastRefreshWarning != "" || status.LastRefreshError != "" {
		t.Fatalf("full refresh did not clear partial runtime state: %#v", status)
	}
}

func TestDoRefreshPartialCampusMergesPreviousCache(t *testing.T) {
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
	svc := newTestServiceWithOverride(t, client, "token")
	now := svc.now()
	seedCache(t, svc, &model.TodayClassrooms{
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
	})

	resp, err := svc.queryAll(context.Background())
	if err != nil {
		t.Fatalf("queryAll() error = %v", err)
	}
	shahe := requireCampusByID(t, resp.Campuses, "04")
	if len(shahe.Buildings) != 1 || shahe.Buildings[0].Name != "旧楼" {
		t.Fatalf("expected previous shahe campus data merged, got %#v", shahe)
	}
}

func TestDoRefreshAllCampusesFail(t *testing.T) {
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			return nil, newJWError(jwErrorQuery, "jw query", nil, "down")
		},
	}
	svc := newTestServiceWithOverride(t, client, "token")

	_, err := svc.queryAll(context.Background())
	if err == nil {
		t.Fatal("queryAll() expected error when all campuses fail")
	}
	if _, ok := svc.getCachedTodayClassrooms(); ok {
		t.Fatal("expected no cache update when all campuses fail")
	}
	status := svc.GetRuntimeStatus()
	if status.LastRefreshError == "" {
		t.Fatalf("expected runtime refresh error after total failure: %#v", status)
	}
}

func TestStalePartialCacheUsesLatestTotalRefreshFailure(t *testing.T) {
	client := &mockJWClient{
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			return nil, newJWError(jwErrorQuery, "jw query", nil, "all campuses down")
		},
	}
	fixed := time.Date(2026, 7, 9, 12, 0, 0, 0, businessLocation)
	clock := newFakeClock(fixed)
	svc := newTestServiceWithOptions(t, client, ClassroomServiceOptions{
		TokenOverride: "token",
		Clock:         clock,
	})
	seedCache(t, svc, &model.TodayClassrooms{
		Date:            fixed.Format("2006-01-02"),
		UpdatedAt:       fixed.Add(-time.Hour),
		ExpiresAt:       fixed.Add(-time.Minute),
		StaleUntil:      endOfDay(fixed),
		Campuses:        []model.CampusInfo{{ID: "01", Name: "西土城"}, {ID: "04", Name: "沙河"}},
		PartialCampuses: []string{"04"},
		Error: &model.APIError{
			Type:    string(jwErrorQuery),
			Message: partialCampusErrorMessage,
		},
	})

	resp, err := svc.GetTodayClassrooms(context.Background())
	if err != nil {
		t.Fatalf("GetTodayClassrooms() error = %v", err)
	}
	if !resp.Stale || resp.Error == nil {
		t.Fatalf("expected stale failure response, got %#v", resp)
	}
	if resp.Error.Message == partialCampusErrorMessage {
		t.Fatalf("latest total failure was masked by old partial warning: %#v", resp.Error)
	}
	if want := staleAPIError(errors.New("failed")).Message; resp.Error.Message != want {
		t.Fatalf("stale failure message = %q, want %q", resp.Error.Message, want)
	}

	backoffResp, err := svc.GetTodayClassrooms(context.Background())
	if err != nil {
		t.Fatalf("GetTodayClassrooms() during backoff error = %v", err)
	}
	if backoffResp.Error == nil || backoffResp.Error.Message == partialCampusErrorMessage {
		t.Fatalf("backoff response masked total failure: %#v", backoffResp)
	}
}
