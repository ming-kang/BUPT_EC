package service

import (
	"BUPT_EC/service/model"
	"BUPT_EC/utils"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type TokenManager struct {
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
		token, err := refreshTokenFunc(m, loginCtx)
		if err != nil {
			recordLoginFailure(err)
			slog.WarnContext(ctx, "jw login failed", "elapsed", time.Since(startedAt), "err", err)
			return "", err
		}
		m.setToken(token)
		recordLoginSuccess(time.Now())
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
		apiURL, err := m.fetchAPIURL(apiCtx)
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

func (m *TokenManager) refreshToken(ctx context.Context) (string, error) {
	requestCtx, cancel := context.WithTimeout(ctx, jwRequestTimeout)
	defer cancel()

	apiURL, err := m.APIURL(requestCtx)
	if err != nil {
		return "", err
	}
	loginURL, err := joinAPIPath(apiURL, "login")
	if err != nil {
		return "", newJWError(jwErrorConfig, "jw login", err, "build login URL failed")
	}

	userNo := os.Getenv(LoginUsernameKey)
	password := os.Getenv(LoginPasswordKey)
	if userNo == "" || password == "" {
		return "", newJWError(jwErrorConfig, "jw login", nil, "JW_USERNAME or JW_PASSWORD is not configured")
	}

	encryptedPassword, err := encryptJWPassword(password)
	if err != nil {
		return "", newJWError(jwErrorConfig, "jw login", err, "encrypt login password failed")
	}
	req := map[string]string{
		"userNo":      userNo,
		"pwd":         encryptedPassword,
		"encode":      "1",
		"captchaData": "",
		"codeVal":     "",
	}

	code, _, body, err := utils.HttpPostForm(requestCtx, loginURL, req)
	if err != nil {
		return "", newJWError(jwErrorLogin, "jw login", err, "request failed")
	}
	if code != http.StatusOK {
		return "", newJWError(jwErrorLogin, "jw login", nil, "http status %d", code)
	}

	var resp model.LoginResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", newJWError(jwErrorParse, "jw login", err, "response invalid")
	}
	if resp.Code != "1" || resp.Data.Token == "" {
		return "", newJWError(jwErrorLogin, "jw login", nil, "%s", safeRemoteMessage(resp.Msg))
	}
	return resp.Data.Token, nil
}

func (m *TokenManager) fetchAPIURL(ctx context.Context) (string, error) {
	requestCtx, cancel := context.WithTimeout(ctx, jwRequestTimeout)
	defer cancel()

	code, _, body, err := utils.HttpGet(requestCtx, ServerConfigURL)
	if err != nil {
		slog.WarnContext(ctx, "serverconfig request failed; using default HTTPS API URL", "err", err)
		return validateJWAPIURL(DefaultAPIURL)
	}
	if code != http.StatusOK {
		slog.WarnContext(ctx, "serverconfig http status; using default HTTPS API URL", "status", code)
		return validateJWAPIURL(DefaultAPIURL)
	}

	var resp model.ServerConfigResponse
	if err := json.Unmarshal(body, &resp); err != nil || resp.APIURL == "" {
		slog.WarnContext(ctx, "serverconfig invalid; using default HTTPS API URL")
		return validateJWAPIURL(DefaultAPIURL)
	}
	apiURL, err := validateJWAPIURL(resp.APIURL)
	if err != nil {
		slog.WarnContext(ctx, "serverconfig API URL rejected; using default HTTPS API URL", "err", err)
		return validateJWAPIURL(DefaultAPIURL)
	}
	return apiURL, nil
}
