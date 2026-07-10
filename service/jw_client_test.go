package service

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"BUPT_EC/config"
	"BUPT_EC/utils"
)

type serviceHTTPDoerFunc func(*http.Request) (*http.Response, error)

func (fn serviceHTTPDoerFunc) Do(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestNewJWClientUsesInjectedCredentials(t *testing.T) {
	t.Setenv(config.JWUsernameKey, "environment-user")
	t.Setenv(config.JWPasswordKey, "environment-password")

	expectedPassword, err := encryptJWPassword("injected-password")
	if err != nil {
		t.Fatalf("encrypt expected password: %v", err)
	}
	doer := serviceHTTPDoerFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read login request: %v", err)
		}
		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse login request: %v", err)
		}
		if values.Get("userNo") != "injected-user" {
			t.Fatal("login request did not use the injected username")
		}
		if values.Get("pwd") != expectedPassword {
			t.Fatal("login request did not use the injected password")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"code":"1","data":{"token":"login-token"}}`)),
		}, nil
	})

	client, err := NewJWClient("injected-user", "injected-password", doer)
	if err != nil {
		t.Fatalf("NewJWClient() error = %v", err)
	}
	token, err := client.Login(context.Background(), DefaultAPIURL)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if token != "login-token" {
		t.Fatal("Login() returned an unexpected token")
	}
}

func TestNewJWClientRejectsNilDoerWithoutLeakingCredentials(t *testing.T) {
	username := "constructor-secret-user"
	password := "constructor-secret-password"
	var typedNilDoer *http.Client
	for _, doer := range []utils.HTTPDoer{nil, typedNilDoer} {
		_, err := NewJWClient(username, password, doer)
		if err == nil {
			t.Fatal("NewJWClient() expected nil doer error")
		}
		if strings.Contains(err.Error(), username) || strings.Contains(err.Error(), password) {
			t.Fatalf("NewJWClient() error leaked credentials: %v", err)
		}
	}
}

func TestJWClientLoginRejectsMissingInjectedCredentialsBeforeRequest(t *testing.T) {
	doer := serviceHTTPDoerFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatal("Login() made an HTTP request without injected credentials")
		return nil, nil
	})
	client, err := NewJWClient("", "", doer)
	if err != nil {
		t.Fatalf("NewJWClient() error = %v", err)
	}
	_, err = client.Login(context.Background(), DefaultAPIURL)
	if err == nil || !isJWErrorKind(err, jwErrorConfig) {
		t.Fatalf("Login() error = %v, want configuration error", err)
	}
}
