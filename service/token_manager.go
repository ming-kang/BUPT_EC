package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type tokenSource int

const (
	tokenSourceNone tokenSource = iota
	tokenSourceOverride
	tokenSourceLogin
)

type tokenOperationResult struct {
	token          string
	source         tokenSource
	loginPerformed bool
}

// authRecoveryDecision is the result of rechecking token state after an auth
// failure inside the token singleflight. When reusable is true, state holds an
// already-installed replacement. Otherwise loginSource is the ObserveLogin
// source for the shared network login that must run next.
type authRecoveryDecision struct {
	state       tokenOperationResult
	reusable    bool
	loginSource string
}

type TokenManager struct {
	jwClient       JWClient
	overrideToken  string
	clock          Clock
	metrics        RuntimeMetrics
	onLoginSuccess func(time.Time)
	onLoginFailure func(error)

	mu                  sync.Mutex
	token               string
	tokenSource         tokenSource
	apiURL              string
	overrideInvalidated bool
	tokenGroup          singleflight.Group
	apiURLGroup         singleflight.Group
}

func (m *TokenManager) now() time.Time {
	if m != nil && m.clock != nil {
		return m.clock.Now()
	}
	return time.Now()
}

func (m *TokenManager) EnsureToken(ctx context.Context, forceRefresh bool) (string, error) {
	ctx = nonNilContext(ctx)
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if !forceRefresh {
		if state := m.cachedTokenState(); state.token != "" {
			return state.token, nil
		}
	}

	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		resultCh := m.tokenGroup.DoChan("jw-token", func() (interface{}, error) {
			if !forceRefresh {
				if state := m.cachedTokenState(); state.token != "" {
					return state, nil
				}
				if state := m.installOverrideToken(); state.token != "" {
					return state, nil
				}
			}
			return m.loginAndStore(ctx, "login")
		})
		result, err := waitTokenResult(ctx, resultCh)
		if err != nil {
			return "", err
		}
		// A force-login caller may have joined a normal EnsureToken operation
		// that only installed or reused a token. Start one new shared operation
		// so forceRefresh still means an actual login.
		if forceRefresh && !result.loginPerformed {
			continue
		}
		return result.token, nil
	}
}

// RefreshAfterAuthFailure coordinates recovery for a specific rejected token.
// State is rechecked inside singleflight so a delayed old request can reuse a
// token already installed by another goroutine instead of logging in again.
func (m *TokenManager) RefreshAfterAuthFailure(ctx context.Context, failedToken string) (string, error) {
	ctx = nonNilContext(ctx)
	if err := ctx.Err(); err != nil {
		return "", err
	}

	resultCh := m.tokenGroup.DoChan("jw-token", func() (interface{}, error) {
		decision := m.prepareAuthRecovery(failedToken)
		if decision.reusable {
			return decision.state, nil
		}
		return m.loginAndStore(ctx, decision.loginSource)
	})
	result, err := waitTokenResult(ctx, resultCh)
	if err != nil {
		return "", err
	}
	return result.token, nil
}

func (m *TokenManager) APIURL(ctx context.Context) (string, error) {
	ctx = nonNilContext(ctx)
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if apiURL := m.cachedAPIURL(); apiURL != "" {
		return apiURL, nil
	}

	resultCh := m.apiURLGroup.DoChan("jw-api-url", func() (interface{}, error) {
		if apiURL := m.cachedAPIURL(); apiURL != "" {
			return apiURL, nil
		}
		apiCtx, cancel := sharedOperationContext(ctx)
		defer cancel()
		apiURL, err := m.jwClient.FetchAPIURL(apiCtx)
		if err != nil {
			return "", err
		}
		m.setAPIURL(apiURL)
		return apiURL, nil
	})
	value, err := waitSingleflightResult(ctx, resultCh)
	if err != nil {
		return "", err
	}
	apiURL, ok := value.(string)
	if !ok || apiURL == "" {
		return "", newJWError(jwErrorConfig, "serverconfig", nil, "unexpected API URL result")
	}
	return apiURL, nil
}

// loginAndStore performs one shared JW network login and records exactly one
// ObserveLogin sample for the operation (success or failed). triggerSource is
// "override" when recovery was caused by a rejected startup JW_TOKEN, else "login".
func (m *TokenManager) loginAndStore(ctx context.Context, triggerSource string) (tokenOperationResult, error) {
	startedAt := m.now()
	loginCtx, cancel := sharedOperationContext(ctx)
	defer cancel()
	token, err := m.login(loginCtx)
	duration := m.elapsedSince(startedAt)
	if err != nil {
		m.observeLogin("failed", triggerSource, duration)
		m.notifyLoginFailure(err)
		slog.WarnContext(loginCtx, "jw login failed", "elapsed", duration, "err", err)
		return tokenOperationResult{}, err
	}
	m.setToken(token, tokenSourceLogin)
	completedAt := m.now()
	duration = completedAt.Sub(startedAt)
	if duration < 0 {
		duration = 0
	}
	m.observeLogin("success", triggerSource, duration)
	m.notifyLoginSuccess(completedAt)
	slog.InfoContext(loginCtx, "jw login succeeded", "elapsed", duration)
	return tokenOperationResult{
		token:          token,
		source:         tokenSourceLogin,
		loginPerformed: true,
	}, nil
}

func (m *TokenManager) login(ctx context.Context) (string, error) {
	apiURL, err := m.APIURL(ctx)
	if err != nil {
		return "", err
	}
	return m.jwClient.Login(ctx, apiURL)
}

func (m *TokenManager) elapsedSince(startedAt time.Time) time.Duration {
	duration := m.now().Sub(startedAt)
	if duration < 0 {
		return 0
	}
	return duration
}

func (m *TokenManager) observeLogin(outcome, source string, duration time.Duration) {
	if m == nil || m.metrics == nil {
		return
	}
	m.metrics.ObserveLogin(outcome, source, duration)
}

func (m *TokenManager) notifyLoginSuccess(at time.Time) {
	if m.onLoginSuccess != nil {
		m.onLoginSuccess(at)
	}
}

func (m *TokenManager) notifyLoginFailure(err error) {
	if m.onLoginFailure != nil {
		m.onLoginFailure(err)
	}
}

func (m *TokenManager) cachedTokenState() tokenOperationResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	return tokenOperationResult{token: m.token, source: m.tokenSource}
}

func (m *TokenManager) installOverrideToken() tokenOperationResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.token != "" {
		return tokenOperationResult{token: m.token, source: m.tokenSource}
	}
	if m.overrideInvalidated {
		return tokenOperationResult{}
	}
	token := m.overrideToken
	if token == "" {
		return tokenOperationResult{}
	}
	m.token = token
	m.tokenSource = tokenSourceOverride
	return tokenOperationResult{token: token, source: tokenSourceOverride}
}

func (m *TokenManager) prepareAuthRecovery(failedToken string) authRecoveryDecision {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.token != "" && m.token != failedToken {
		return authRecoveryDecision{
			state:    tokenOperationResult{token: m.token, source: m.tokenSource},
			reusable: true,
		}
	}
	// Capture provenance before clearing so ObserveLogin source reflects the
	// rejected token, not the empty post-clear state.
	loginSource := "login"
	if m.token == failedToken {
		if m.tokenSource == tokenSourceOverride {
			m.overrideInvalidated = true
			loginSource = "override"
		}
		m.token = ""
		m.tokenSource = tokenSourceNone
	}
	return authRecoveryDecision{loginSource: loginSource}
}

func (m *TokenManager) setToken(token string, source tokenSource) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.token = token
	m.tokenSource = source
}

func (m *TokenManager) cachedAPIURL() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.apiURL
}

func (m *TokenManager) setAPIURL(apiURL string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.apiURL = apiURL
}

func nonNilContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func sharedOperationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(nonNilContext(ctx)), jwRequestTimeout)
}

func waitSingleflightResult(ctx context.Context, resultCh <-chan singleflight.Result) (interface{}, error) {
	select {
	case result := <-resultCh:
		return result.Val, result.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func waitTokenResult(ctx context.Context, resultCh <-chan singleflight.Result) (tokenOperationResult, error) {
	value, err := waitSingleflightResult(ctx, resultCh)
	if err != nil {
		return tokenOperationResult{}, err
	}
	result, ok := value.(tokenOperationResult)
	if !ok || result.token == "" {
		return tokenOperationResult{}, newJWError(jwErrorLogin, "jw login", nil, "unexpected token result")
	}
	return result, nil
}
