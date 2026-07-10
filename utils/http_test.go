package utils

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type httpDoerFunc func(*http.Request) (*http.Response, error)

func (fn httpDoerFunc) Do(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestHTTPClientDoesNotFollowRedirectToDisallowedHost(t *testing.T) {
	evilHit := make(chan struct{}, 1)
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case evilHit <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(evil.Close)

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL+"/steal", http.StatusFound)
	}))
	t.Cleanup(origin.Close)

	_, _, _, err := HttpGet(NewHTTPClient(), context.Background(), origin.URL)
	if err == nil {
		t.Fatal("HttpGet: expected error when server returns redirect")
	}
	if !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("HttpGet: error %q should mention redirect", err)
	}

	select {
	case <-evilHit:
		t.Fatal("redirect was followed to disallowed host")
	default:
	}
}

func TestHTTPClientDoesNotFollow307WithTokenHeader(t *testing.T) {
	evilHit := make(chan struct{}, 1)
	var evilToken string
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		evilToken = r.Header.Get("token")
		select {
		case evilHit <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(evil.Close)

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", evil.URL+"/steal")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	t.Cleanup(origin.Close)

	_, _, _, err := HttpPostWithHeader(NewHTTPClient(), context.Background(), origin.URL, map[string]string{
		"token": "secret-jw-token",
	})
	if err == nil {
		t.Fatal("HttpPostWithHeader: expected error when server returns 307")
	}
	if !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("HttpPostWithHeader: error %q should mention redirect", err)
	}

	select {
	case <-evilHit:
		if evilToken != "" {
			t.Fatal("307 redirect forwarded the token header to a disallowed host")
		}
		t.Fatal("307 redirect was followed to a disallowed host")
	default:
	}
}

func TestHTTPHelpersUseProvidedDoer(t *testing.T) {
	called := false
	doer := httpDoerFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		if req.Method != http.MethodGet || req.URL.String() != "https://example.test/path" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		}
		if req.Header.Get("Accept") == "" {
			t.Fatal("request is missing default Accept header")
		}
		return &http.Response{
			StatusCode: http.StatusCreated,
			Header:     http.Header{"X-Test": []string{"ok"}},
			Body:       io.NopCloser(strings.NewReader("response")),
		}, nil
	})

	status, headers, body, err := HttpGet(doer, context.Background(), "https://example.test/path")
	if err != nil {
		t.Fatalf("HttpGet() error = %v", err)
	}
	if !called || status != http.StatusCreated || headers.Get("X-Test") != "ok" || string(body) != "response" {
		t.Fatalf("HttpGet() result = called:%v status:%d headers:%v body:%q", called, status, headers, body)
	}
}

func TestHTTPResponseBodyLimit(t *testing.T) {
	doer := httpDoerFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(io.LimitReader(strings.NewReader(strings.Repeat("x", int(maxResponseBodyBytes)+1)), maxResponseBodyBytes+1)),
		}, nil
	})

	_, _, _, err := HttpGet(doer, context.Background(), "https://example.test/large")
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("HttpGet() body-limit error = %v", err)
	}
}

func TestNewHTTPClientPreservesTransportSettings(t *testing.T) {
	first := NewHTTPClient()
	second := NewHTTPClient()
	if first == second {
		t.Fatal("NewHTTPClient() returned the same client instance")
	}
	if first.Transport == second.Transport {
		t.Fatal("NewHTTPClient() returned clients sharing one transport instance")
	}
	transport, ok := first.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport type = %T, want *http.Transport", first.Transport)
	}
	if transport.MaxIdleConns != 100 || transport.MaxIdleConnsPerHost != 10 ||
		transport.IdleConnTimeout != 90*time.Second || transport.TLSHandshakeTimeout != 5*time.Second ||
		transport.ResponseHeaderTimeout != 12*time.Second || transport.ExpectContinueTimeout != time.Second {
		t.Fatalf("unexpected HTTP transport settings: %#v", transport)
	}
	if transport.Proxy == nil {
		t.Fatal("NewHTTPClient() is missing environment proxy support")
	}
	if first.CheckRedirect == nil {
		t.Fatal("NewHTTPClient() is missing redirect protection")
	}
}

func TestHTTPHelpersRejectNilDoer(t *testing.T) {
	var typedNilDoer *http.Client
	for _, doer := range []HTTPDoer{nil, typedNilDoer} {
		_, _, _, err := HttpGet(doer, context.Background(), "https://example.test")
		if err == nil {
			t.Fatal("HttpGet() expected nil doer error")
		}
	}
}

func TestCheckRedirectRejectsAllTargets(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://jwglweixin.bupt.edu.cn/next", nil)
	if err != nil {
		t.Fatal(err)
	}
	via := []*http.Request{req}
	if err := CheckRedirect(req, via); err == nil {
		t.Fatal("CheckRedirect: expected error for any redirect target")
	}
}
