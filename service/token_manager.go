package service

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type TokenManager struct {
	jwClient       JWClient
	onLoginSuccess func(time.Time)
	onLoginFailure func(error)

	mu          sync.Mutex
	token       string
	apiURL      string
	tokenGroup  singleflight.Group
	apiURLGroup singleflight.Group
}

func (m *TokenManager) EnsureToken(ctx context.Context, forceRefresh bool) (string, error) {
	if !forceRefresh {
		if token := os.Getenv(LoginTokenKey); token != "" {
			m.setToken(token)
			return token, nil
		}
		if token := m.cachedToken(); token != "" {
			return token, nil
		}
	}

	value, err, _ := m.tokenGroup.Do("jw-token", func() (interface{}, error) {
		if !forceRefresh {
			if token := os.Getenv(LoginTokenKey); token != "" {
				m.setToken(token)
				return token, nil
			}
			if token := m.cachedToken(); token != "" {
				return token, nil
			}
		}

		startedAt := time.Now()
		loginCtx, cancel := context.WithTimeout(context.Background(), jwRequestTimeout)
		defer cancel()
		token, err := m.login(loginCtx)
		if err != nil {
			m.notifyLoginFailure(err)
			slog.WarnContext(ctx, "jw login failed", "elapsed", time.Since(startedAt), "err", err)
			return "", err
		}
		m.setToken(token)
		m.notifyLoginSuccess(time.Now())
		slog.InfoContext(ctx, "jw login succeeded", "elapsed", time.Since(startedAt))
		return token, nil
	})
	if err != nil {
		return "", err
	}
	token, ok := value.(string)
	if !ok || token == "" {
		return "", newJWError(jwErrorLogin, "jw login", nil, "unexpected token result")
	}
	return token, nil
}

func (m *TokenManager) APIURL(ctx context.Context) (string, error) {
	if apiURL := m.cachedAPIURL(); apiURL != "" {
		return apiURL, nil
	}

	value, err, _ := m.apiURLGroup.Do("jw-api-url", func() (interface{}, error) {
		if apiURL := m.cachedAPIURL(); apiURL != "" {
			return apiURL, nil
		}
		apiCtx, cancel := context.WithTimeout(context.Background(), jwRequestTimeout)
		defer cancel()
		apiURL, err := m.jwClient.FetchAPIURL(apiCtx)
		if err != nil {
			return "", err
		}
		m.setAPIURL(apiURL)
		return apiURL, nil
	})
	if err != nil {
		return "", err
	}
	apiURL, ok := value.(string)
	if !ok || apiURL == "" {
		return "", newJWError(jwErrorConfig, "serverconfig", nil, "unexpected API URL result")
	}
	return apiURL, nil
}

func (m *TokenManager) login(ctx context.Context) (string, error) {
	apiURL, err := m.APIURL(ctx)
	if err != nil {
		return "", err
	}
	return m.jwClient.Login(ctx, apiURL)
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

func (m *TokenManager) cachedToken() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.token
}

func (m *TokenManager) setToken(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.token = token
}

func (m *TokenManager) clearTokenIfCurrent(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.token == token {
		m.token = ""
	}
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
