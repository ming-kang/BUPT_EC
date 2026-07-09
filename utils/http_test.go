package utils

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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

	_, _, _, err := HttpGet(context.Background(), origin.URL)
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

	_, _, _, err := HttpPostWithHeader(context.Background(), origin.URL, map[string]string{
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
		t.Fatalf("307 redirect was followed to disallowed host (token=%q)", evilToken)
	default:
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
