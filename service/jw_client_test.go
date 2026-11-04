package service

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type serviceHTTPDoerFunc func(*http.Request) (*http.Response, error)

func (fn serviceHTTPDoerFunc) Do(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestNewJWClientUsesInjectedCredentials(t *testing.T) {
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

	client := NewJWClient("injected-user", "injected-password", doer)
	token, err := client.Login(context.Background(), DefaultAPIURL)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if token != "login-token" {
		t.Fatal("Login() returned an unexpected token")
	}
}
