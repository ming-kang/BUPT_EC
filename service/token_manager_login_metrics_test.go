package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type loginObservation struct {
	outcome  string
	source   string
	duration time.Duration
}

type recordingLoginMetrics struct {
	NoopMetrics
	mu   sync.Mutex
	logs []loginObservation
}

func (m *recordingLoginMetrics) ObserveLogin(outcome, source string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, loginObservation{
		outcome:  outcome,
		source:   source,
		duration: duration,
	})
}

func (m *recordingLoginMetrics) snapshot() []loginObservation {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]loginObservation, len(m.logs))
	copy(out, m.logs)
	return out
}

type sequenceClock struct {
	mu    sync.Mutex
	times []time.Time
	idx   int
}

func (c *sequenceClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.times) == 0 {
		return time.Unix(0, 0).UTC()
	}
	if c.idx >= len(c.times) {
		return c.times[len(c.times)-1]
	}
	now := c.times[c.idx]
	c.idx++
	return now
}

func TestLoginMetricsFirstLoginSuccess(t *testing.T) {
	metrics := &recordingLoginMetrics{}
	start := time.Unix(1_700_000_000, 0).UTC()
	clock := &sequenceClock{times: []time.Time{
		start,
		start.Add(120 * time.Millisecond),
	}}
	manager := &TokenManager{
		metrics: metrics,
		clock:   clock,
		jwClient: &mockJWClient{
			login: func(ctx context.Context, apiURL string) (string, error) {
				return "login-token", nil
			},
		},
	}

	token, err := manager.EnsureToken(context.Background(), false)
	if err != nil {
		t.Fatalf("EnsureToken() error = %v", err)
	}
	if token != "login-token" {
		t.Fatalf("token = %q, want login-token", token)
	}

	logs := metrics.snapshot()
	if len(logs) != 1 {
		t.Fatalf("observations = %d, want 1: %#v", len(logs), logs)
	}
	if logs[0].outcome != "success" || logs[0].source != "login" {
		t.Fatalf("observation = %#v, want success/login", logs[0])
	}
	if logs[0].duration != 120*time.Millisecond {
		t.Fatalf("duration = %v, want 120ms", logs[0].duration)
	}

	// Cache hit must not observe again.
	if _, err := manager.EnsureToken(context.Background(), false); err != nil {
		t.Fatalf("second EnsureToken() error = %v", err)
	}
	if got := len(metrics.snapshot()); got != 1 {
		t.Fatalf("after cache hit observations = %d, want 1", got)
	}
}

func TestLoginMetricsFailureDoesNotLeakLabels(t *testing.T) {
	metrics := &recordingLoginMetrics{}
	manager := &TokenManager{
		metrics: metrics,
		jwClient: &mockJWClient{
			login: func(ctx context.Context, apiURL string) (string, error) {
				return "", newJWError(jwErrorLogin, "jw login", nil, "token=super-secret user=alice https://jw.example")
			},
		},
	}

	_, err := manager.EnsureToken(context.Background(), false)
	if err == nil {
		t.Fatal("EnsureToken() expected error")
	}
	logs := metrics.snapshot()
	if len(logs) != 1 {
		t.Fatalf("observations = %d, want 1: %#v", len(logs), logs)
	}
	if logs[0].outcome != "failed" || logs[0].source != "login" {
		t.Fatalf("observation = %#v, want failed/login", logs[0])
	}
	if logs[0].duration < 0 {
		t.Fatalf("duration = %v, want non-negative", logs[0].duration)
	}
}

func TestLoginMetricsOverrideRecoverySource(t *testing.T) {
	metrics := &recordingLoginMetrics{}
	manager := &TokenManager{
		overrideToken: "override-token",
		metrics:       metrics,
		jwClient: &mockJWClient{
			login: func(ctx context.Context, apiURL string) (string, error) {
				return "login-token", nil
			},
		},
	}

	override, err := manager.EnsureToken(context.Background(), false)
	if err != nil {
		t.Fatalf("EnsureToken() error = %v", err)
	}
	if override != "override-token" {
		t.Fatalf("override = %q", override)
	}
	if got := len(metrics.snapshot()); got != 0 {
		t.Fatalf("override install observations = %d, want 0", got)
	}

	if _, err := manager.RefreshAfterAuthFailure(context.Background(), override); err != nil {
		t.Fatalf("RefreshAfterAuthFailure() error = %v", err)
	}
	logs := metrics.snapshot()
	if len(logs) != 1 {
		t.Fatalf("observations = %d, want 1: %#v", len(logs), logs)
	}
	if logs[0].outcome != "success" || logs[0].source != "override" {
		t.Fatalf("observation = %#v, want success/override", logs[0])
	}
}

func TestLoginMetricsLoginTokenRecoverySource(t *testing.T) {
	metrics := &recordingLoginMetrics{}
	manager := &TokenManager{
		metrics: metrics,
		jwClient: &mockJWClient{
			login: func(ctx context.Context, apiURL string) (string, error) {
				return "replacement-token", nil
			},
		},
	}
	manager.setToken("expired-login-token", tokenSourceLogin)

	if _, err := manager.RefreshAfterAuthFailure(context.Background(), "expired-login-token"); err != nil {
		t.Fatalf("RefreshAfterAuthFailure() error = %v", err)
	}
	logs := metrics.snapshot()
	if len(logs) != 1 {
		t.Fatalf("observations = %d, want 1: %#v", len(logs), logs)
	}
	if logs[0].outcome != "success" || logs[0].source != "login" {
		t.Fatalf("observation = %#v, want success/login", logs[0])
	}
}

func TestLoginMetricsConcurrentWaitersObserveOnce(t *testing.T) {
	metrics := &recordingLoginMetrics{}
	loginStarted := make(chan struct{})
	releaseLogin := make(chan struct{})
	var once sync.Once
	var loginCalls atomic.Int32
	manager := &TokenManager{
		metrics: metrics,
		jwClient: &mockJWClient{
			login: func(ctx context.Context, apiURL string) (string, error) {
				loginCalls.Add(1)
				once.Do(func() { close(loginStarted) })
				select {
				case <-releaseLogin:
					return "shared-token", nil
				case <-ctx.Done():
					return "", ctx.Err()
				}
			},
		},
	}

	const waiters = 8
	errCh := make(chan error, waiters)
	for range waiters {
		go func() {
			token, err := manager.EnsureToken(context.Background(), false)
			if err != nil {
				errCh <- err
				return
			}
			if token != "shared-token" {
				errCh <- fmt.Errorf("token = %q", token)
				return
			}
			errCh <- nil
		}()
	}
	select {
	case <-loginStarted:
	case <-time.After(time.Second):
		t.Fatal("shared login did not start")
	}
	close(releaseLogin)
	for range waiters {
		if err := <-errCh; err != nil {
			t.Fatalf("waiter error = %v", err)
		}
	}
	if got := loginCalls.Load(); got != 1 {
		t.Fatalf("login calls = %d, want 1", got)
	}
	if got := len(metrics.snapshot()); got != 1 {
		t.Fatalf("observations = %d, want 1: %#v", got, metrics.snapshot())
	}
}

func TestLoginMetricsDelayedFailureReusesReplacementWithoutObservation(t *testing.T) {
	metrics := &recordingLoginMetrics{}
	manager := &TokenManager{
		metrics: metrics,
		jwClient: &mockJWClient{
			login: func(ctx context.Context, apiURL string) (string, error) {
				return "fresh-token", nil
			},
		},
	}
	manager.setToken("expired-token", tokenSourceLogin)

	if _, err := manager.RefreshAfterAuthFailure(context.Background(), "expired-token"); err != nil {
		t.Fatalf("first recovery error = %v", err)
	}
	if got := len(metrics.snapshot()); got != 1 {
		t.Fatalf("after first recovery observations = %d, want 1", got)
	}

	// Delayed old failure must reuse the installed replacement without login.
	token, err := manager.RefreshAfterAuthFailure(context.Background(), "expired-token")
	if err != nil {
		t.Fatalf("delayed recovery error = %v", err)
	}
	if token != "fresh-token" {
		t.Fatalf("token = %q, want fresh-token", token)
	}
	if got := len(metrics.snapshot()); got != 1 {
		t.Fatalf("after delayed reuse observations = %d, want 1", got)
	}
}

func TestLoginMetricsNegativeDurationClamped(t *testing.T) {
	metrics := &recordingLoginMetrics{}
	// Clock goes backwards between start and end.
	start := time.Unix(1_700_000_100, 0).UTC()
	clock := &sequenceClock{times: []time.Time{
		start,
		start.Add(-5 * time.Second),
	}}
	manager := &TokenManager{
		metrics: metrics,
		clock:   clock,
		jwClient: &mockJWClient{
			login: func(ctx context.Context, apiURL string) (string, error) {
				return "token", nil
			},
		},
	}
	if _, err := manager.EnsureToken(context.Background(), false); err != nil {
		t.Fatalf("EnsureToken() error = %v", err)
	}
	logs := metrics.snapshot()
	if len(logs) != 1 {
		t.Fatalf("observations = %d, want 1", len(logs))
	}
	if logs[0].duration != 0 {
		t.Fatalf("duration = %v, want 0 (clamped)", logs[0].duration)
	}
}

func TestLoginMetricsAPIURLFailureIsFailedObservation(t *testing.T) {
	metrics := &recordingLoginMetrics{}
	manager := &TokenManager{
		metrics: metrics,
		jwClient: &mockJWClient{
			fetchAPIURL: func(ctx context.Context) (string, error) {
				return "", errors.New("serverconfig down")
			},
		},
	}
	if _, err := manager.EnsureToken(context.Background(), false); err == nil {
		t.Fatal("EnsureToken() expected API URL failure")
	}
	logs := metrics.snapshot()
	if len(logs) != 1 {
		t.Fatalf("observations = %d, want 1: %#v", len(logs), logs)
	}
	if logs[0].outcome != "failed" || logs[0].source != "login" {
		t.Fatalf("observation = %#v, want failed/login", logs[0])
	}
}

func TestLoginMetricsNilMetricsSafe(t *testing.T) {
	manager := &TokenManager{
		jwClient: &mockJWClient{
			login: func(ctx context.Context, apiURL string) (string, error) {
				return "token", nil
			},
		},
	}
	if _, err := manager.EnsureToken(context.Background(), false); err != nil {
		t.Fatalf("EnsureToken() with nil metrics error = %v", err)
	}
}
