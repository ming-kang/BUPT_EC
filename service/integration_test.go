//go:build integration

package service

// Real-network integration tests against the BUPT JW upstream. They compile
// only with `go test -tags integration ./service` and skip themselves when the
// JW credential environment variables are absent.

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func requireJWCredentials(t *testing.T) (username, password string) {
	t.Helper()
	username, password = os.Getenv("JW_USERNAME"), os.Getenv("JW_PASSWORD")
	if username == "" || password == "" {
		t.Skip("JW_USERNAME/JW_PASSWORD are required for integration tests")
	}
	return username, password
}

func TestRefreshAgainstLiveJW(t *testing.T) {
	username, password := requireJWCredentials(t)
	svc, _ := newTestService(t, newHTTPJWClientForTest(t, username, password))
	svc.clock = time.Now
	if err := svc.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := svc.cache.Load(); got == nil || len(got.Campuses) != 2 || got.Error != nil {
		t.Fatalf("live refresh snapshot = %+v", got)
	}
}

// TestJWRoomTokensCarryPositiveCapacitySuffix is the live wire-contract guard
// for the classroom display semantics: RoomInfo.Capacity is parsed from
// the trailing `(N)` of each CLASSROOMS token (classroom_builder.go), and the
// frontend's 未知 fallback is only meant for malformed tokens — which also land
// in the 未分组 building.
func TestJWRoomTokensCarryPositiveCapacitySuffix(t *testing.T) {
	username, password := requireJWCredentials(t)
	client := newHTTPJWClientForTest(t, username, password)
	apiURL, err := client.FetchAPIURL(context.Background())
	if err != nil {
		t.Fatalf("FetchAPIURL() error = %v", err)
	}
	token, err := client.Login(context.Background(), apiURL)
	if err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile(`^(.+)[(（](\d+)[)）]$`)
	totalTokens := 0
	for _, campusID := range []string{"01", "04"} {
		rows, err := client.QueryCampus(context.Background(), apiURL, campusID, token)
		if err != nil {
			t.Fatalf("QueryCampus(%s) error = %v", campusID, err)
		}
		for _, row := range rows {
			for _, raw := range strings.Split(row.Classrooms, ",") {
				raw = strings.TrimSpace(raw)
				if raw == "" {
					continue
				}
				totalTokens++
				matches := pattern.FindStringSubmatch(raw)
				if matches == nil {
					t.Errorf("campus %s: CLASSROOMS token %q carries no (N) capacity suffix", campusID, raw)
					continue
				}
				if capacity, err := strconv.Atoi(matches[2]); err == nil && capacity == 0 {
					t.Errorf("campus %s: CLASSROOMS token %q carries zero capacity", campusID, raw)
				}
			}
		}
	}
	if totalTokens == 0 {
		t.Error("expected non-empty CLASSROOMS tokens from live JW")
	}
}
