//go:build integration

package service

// Real-network integration tests against the BUPT JW upstream. They compile
// only with `go test -tags integration ./service` and skip themselves when the
// JW credential environment variables are absent.

import (
	"context"
	"os"
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
