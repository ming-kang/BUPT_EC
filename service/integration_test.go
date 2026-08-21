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

	"BUPT_EC/config"
)

func newIntegrationService(t *testing.T) *ClassroomService {
	t.Helper()
	return newTestServiceWithOverride(t,
		newHTTPJWClientForTest(t, os.Getenv(config.JWUsernameKey), os.Getenv(config.JWPasswordKey)),
		os.Getenv(config.JWTokenKey),
	)
}

func requireJWCredentials(t *testing.T) {
	t.Helper()
	if os.Getenv(config.JWTokenKey) != "" {
		return
	}
	if os.Getenv(config.JWUsernameKey) == "" || os.Getenv(config.JWPasswordKey) == "" {
		t.Skip("JW_TOKEN or JW_USERNAME/JW_PASSWORD are required for integration login/query tests")
	}
}

func requireJWLoginCredentials(t *testing.T) {
	t.Helper()
	if os.Getenv(config.JWUsernameKey) == "" || os.Getenv(config.JWPasswordKey) == "" {
		t.Skip("JW_USERNAME/JW_PASSWORD are required for the integration login test")
	}
}

func TestLogin(t *testing.T) {
	requireJWLoginCredentials(t)
	client := newHTTPJWClientForTest(t, os.Getenv(config.JWUsernameKey), os.Getenv(config.JWPasswordKey))
	apiURL, err := client.FetchAPIURL(context.Background())
	if err != nil {
		t.Fatalf("FetchAPIURL() error = %v", err)
	}
	token, err := client.Login(context.Background(), apiURL)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Error("expected a non-empty login token")
	}
}

func TestQueryOne(t *testing.T) {
	requireJWCredentials(t)
	svc := newIntegrationService(t)
	rows, err := svc.queryOne(context.Background(), "01")
	if err != nil {
		t.Error(err)
	}
	if len(rows) == 0 {
		t.Error("expected classroom rows")
	}
}

func TestQueryAll(t *testing.T) {
	requireJWCredentials(t)
	svc := newIntegrationService(t)
	ans, err := svc.queryAll(context.Background())
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

// TestJWRoomTokensCarryPositiveCapacitySuffix is the live wire-contract guard
// for the classroom display semantics (F-05): RoomInfo.Capacity is parsed from
// the trailing `(N)` of each CLASSROOMS token (classroom_builder.go), and the
// frontend's 未知 fallback is only meant for malformed tokens — which also land
// in the 未分组 building. If JW ever emits suffix-less or zero-capacity tokens,
// this test fails and both the builder assumptions and api-contract.md need
// revisiting.
func TestJWRoomTokensCarryPositiveCapacitySuffix(t *testing.T) {
	requireJWCredentials(t)
	client := newHTTPJWClientForTest(t, os.Getenv(config.JWUsernameKey), os.Getenv(config.JWPasswordKey))
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
