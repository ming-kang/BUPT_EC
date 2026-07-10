package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"BUPT_EC/service/model"
)

func tokenTestRows(campusID string) []model.JWClassInfo {
	return []model.JWClassInfo{{
		NodeName:   "1",
		NodeTime:   "08:00-08:45",
		Classrooms: fmt.Sprintf("教学实验综合楼-%s(10)", campusID),
	}}
}

func TestConcurrentAuthFailuresShareOneLogin(t *testing.T) {
	var oldQueries atomic.Int32
	var loginCalls atomic.Int32
	bothOldQueries := make(chan struct{})
	releaseOldQueries := make(chan struct{})
	client := &mockJWClient{
		login: func(ctx context.Context, apiURL string) (string, error) {
			loginCalls.Add(1)
			return "fresh-token", nil
		},
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			if token == "expired-token" {
				if oldQueries.Add(1) == 2 {
					close(bothOldQueries)
				}
				select {
				case <-releaseOldQueries:
					return nil, newJWError(jwErrorAuth, "jw query", nil, "expired")
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			if token != "fresh-token" {
				return nil, newJWError(jwErrorAuth, "jw query", nil, "unexpected test token")
			}
			return tokenTestRows(campusID), nil
		},
	}
	svc := newTestService(t, client)
	svc.tokenManager.setToken("expired-token", tokenSourceLogin)

	errCh := make(chan error, 2)
	for _, campusID := range []string{"01", "04"} {
		campusID := campusID
		go func() {
			_, err := svc.queryCampus(context.Background(), campusID)
			errCh <- err
		}()
	}
	select {
	case <-bothOldQueries:
	case <-time.After(time.Second):
		t.Fatal("campus queries did not use the same expired token concurrently")
	}
	close(releaseOldQueries)
	for range 2 {
		if err := <-errCh; err != nil {
			t.Fatalf("queryCampus() error = %v", err)
		}
	}
	if got := loginCalls.Load(); got != 1 {
		t.Fatalf("login calls = %d, want 1", got)
	}
}

func TestDelayedAuthFailureReusesInstalledToken(t *testing.T) {
	var oldQueries atomic.Int32
	var loginCalls atomic.Int32
	var newTokenUsed sync.Once
	bothOldQueries := make(chan struct{})
	firstRetryComplete := make(chan struct{})
	client := &mockJWClient{
		login: func(ctx context.Context, apiURL string) (string, error) {
			call := loginCalls.Add(1)
			return fmt.Sprintf("fresh-token-%d", call), nil
		},
		queryCampus: func(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
			if token == "expired-token" {
				if oldQueries.Add(1) == 2 {
					close(bothOldQueries)
				}
				select {
				case <-bothOldQueries:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				if campusID == "04" {
					select {
					case <-firstRetryComplete:
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				}
				return nil, newJWError(jwErrorAuth, "jw query", nil, "expired")
			}
			if token != "fresh-token-1" {
				return nil, newJWError(jwErrorAuth, "jw query", nil, "superseded")
			}
			if campusID == "01" {
				newTokenUsed.Do(func() { close(firstRetryComplete) })
			}
			return tokenTestRows(campusID), nil
		},
	}
	svc := newTestService(t, client)
	svc.tokenManager.setToken("expired-token", tokenSourceLogin)

	errCh := make(chan error, 2)
	for _, campusID := range []string{"01", "04"} {
		campusID := campusID
		go func() {
			_, err := svc.queryCampus(context.Background(), campusID)
			errCh <- err
		}()
	}
	for range 2 {
		if err := <-errCh; err != nil {
			t.Fatalf("queryCampus() error = %v", err)
		}
	}
	if got := loginCalls.Load(); got != 1 {
		t.Fatalf("delayed auth failure triggered %d logins, want 1", got)
	}
}

func TestAuthFailureInvalidatesOnlyRejectedOverrideSource(t *testing.T) {
	var loginCalls atomic.Int32
	manager := &TokenManager{
		overrideToken: "override-token",
		jwClient: &mockJWClient{
			login: func(ctx context.Context, apiURL string) (string, error) {
				loginCalls.Add(1)
				return "login-token", nil
			},
		},
	}

	override, err := manager.EnsureToken(context.Background(), false)
	if err != nil {
		t.Fatalf("EnsureToken() error = %v", err)
	}
	if override != "override-token" {
		t.Fatal("expected environment override")
	}
	if _, err := manager.RefreshAfterAuthFailure(context.Background(), override); err != nil {
		t.Fatalf("RefreshAfterAuthFailure() error = %v", err)
	}
	manager.mu.Lock()
	invalidated := manager.overrideInvalidated
	source := manager.tokenSource
	manager.mu.Unlock()
	if !invalidated || source != tokenSourceLogin {
		t.Fatalf("override recovery state = invalidated:%v source:%v", invalidated, source)
	}
	if got := loginCalls.Load(); got != 1 {
		t.Fatalf("login calls = %d, want 1", got)
	}
}

func TestLoginTokenFailurePreservesOverrideInvalidationState(t *testing.T) {
	manager := &TokenManager{
		overrideToken: "old-override",
		jwClient: &mockJWClient{
			login: func(ctx context.Context, apiURL string) (string, error) {
				return "new-login-token", nil
			},
		},
	}
	manager.mu.Lock()
	manager.overrideInvalidated = true
	manager.mu.Unlock()
	manager.setToken("expired-login-token", tokenSourceLogin)

	if _, err := manager.RefreshAfterAuthFailure(context.Background(), "expired-login-token"); err != nil {
		t.Fatalf("RefreshAfterAuthFailure() error = %v", err)
	}
	manager.mu.Lock()
	invalidated := manager.overrideInvalidated
	source := manager.tokenSource
	manager.mu.Unlock()
	if !invalidated {
		t.Fatal("login-token recovery restored an invalidated override")
	}
	if source != tokenSourceLogin {
		t.Fatalf("token source = %v, want login", source)
	}
}

func TestCanceledTokenWaiterDoesNotCancelSharedLogin(t *testing.T) {
	loginStarted := make(chan struct{})
	releaseLogin := make(chan struct{})
	var once sync.Once
	var loginCalls atomic.Int32
	manager := &TokenManager{
		jwClient: &mockJWClient{
			login: func(ctx context.Context, apiURL string) (string, error) {
				loginCalls.Add(1)
				once.Do(func() { close(loginStarted) })
				select {
				case <-releaseLogin:
					return "shared-login-token", nil
				case <-ctx.Done():
					return "", ctx.Err()
				}
			},
		},
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	canceledResult := make(chan error, 1)
	go func() {
		_, err := manager.EnsureToken(canceledCtx, false)
		canceledResult <- err
	}()
	select {
	case <-loginStarted:
	case <-time.After(time.Second):
		t.Fatal("shared login did not start")
	}
	survivorResult := make(chan error, 1)
	go func() {
		token, err := manager.EnsureToken(context.Background(), false)
		if err == nil && token != "shared-login-token" {
			err = fmt.Errorf("unexpected shared login result")
		}
		survivorResult <- err
	}()

	cancel()
	select {
	case err := <-canceledResult:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled waiter error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled token waiter did not return promptly")
	}
	close(releaseLogin)
	if err := <-survivorResult; err != nil {
		t.Fatalf("surviving waiter error = %v", err)
	}
	if got := loginCalls.Load(); got != 1 {
		t.Fatalf("login calls = %d, want 1", got)
	}
}

func TestCanceledAPIURLWaiterDoesNotCancelSharedFetch(t *testing.T) {
	fetchStarted := make(chan struct{})
	releaseFetch := make(chan struct{})
	var once sync.Once
	var fetchCalls atomic.Int32
	manager := &TokenManager{
		jwClient: &mockJWClient{
			fetchAPIURL: func(ctx context.Context) (string, error) {
				fetchCalls.Add(1)
				once.Do(func() { close(fetchStarted) })
				select {
				case <-releaseFetch:
					return DefaultAPIURL, nil
				case <-ctx.Done():
					return "", ctx.Err()
				}
			},
		},
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	canceledResult := make(chan error, 1)
	go func() {
		_, err := manager.APIURL(canceledCtx)
		canceledResult <- err
	}()
	select {
	case <-fetchStarted:
	case <-time.After(time.Second):
		t.Fatal("shared API URL fetch did not start")
	}
	survivorResult := make(chan error, 1)
	go func() {
		apiURL, err := manager.APIURL(context.Background())
		if err == nil && apiURL != DefaultAPIURL {
			err = fmt.Errorf("unexpected API URL result")
		}
		survivorResult <- err
	}()

	cancel()
	select {
	case err := <-canceledResult:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled waiter error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled API URL waiter did not return promptly")
	}
	close(releaseFetch)
	if err := <-survivorResult; err != nil {
		t.Fatalf("surviving waiter error = %v", err)
	}
	if got := fetchCalls.Load(); got != 1 {
		t.Fatalf("FetchAPIURL calls = %d, want 1", got)
	}
}
