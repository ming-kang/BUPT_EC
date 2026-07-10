package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Local protocol fixtures for JWClient. These never touch the network: the
// injected HTTPDoer captures method/URL/headers/body and returns canned
// responses. CI must keep this path non-skipping and credential-free.

func TestJWClientQueryCampusProtocolContract(t *testing.T) {
	const (
		token    = "fixture-token"
		campusID = "04"
		apiURL   = "https://jwglweixin.bupt.edu.cn/api/"
	)

	var sawMethod, sawURL, sawToken string
	doer := serviceHTTPDoerFunc(func(req *http.Request) (*http.Response, error) {
		sawMethod = req.Method
		sawURL = req.URL.String()
		sawToken = req.Header.Get("token")
		body := `{"code":"1","Msg":"ok","data":[{"CLASSROOMS":"教学楼-101(40)","NODETIME":"08:00-09:35","NODENAME":"第1-2节"}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})

	client, err := NewJWClient("fixture-user", "fixture-pass", doer)
	if err != nil {
		t.Fatalf("NewJWClient() error = %v", err)
	}
	rows, err := client.QueryCampus(context.Background(), apiURL, campusID, token)
	if err != nil {
		t.Fatalf("QueryCampus() error = %v", err)
	}
	if sawMethod != http.MethodPost {
		t.Fatalf("QueryCampus method = %q, want POST", sawMethod)
	}
	parsed, err := url.Parse(sawURL)
	if err != nil {
		t.Fatalf("parse captured URL: %v", err)
	}
	if !strings.HasSuffix(parsed.Path, "/todayClassrooms") {
		t.Fatalf("QueryCampus path = %q, want .../todayClassrooms", parsed.Path)
	}
	if parsed.Query().Get("campusId") != campusID {
		t.Fatalf("QueryCampus campusId = %q, want %q", parsed.Query().Get("campusId"), campusID)
	}
	if sawToken != token {
		t.Fatalf("QueryCampus token header = %q, want %q", sawToken, token)
	}
	if len(rows) != 1 || rows[0].Classrooms != "教学楼-101(40)" {
		t.Fatalf("QueryCampus rows = %#v, want one classroom row", rows)
	}
}

func TestJWClientQueryCampusClassifiesAuthAndParseFailures(t *testing.T) {
	apiURL := "https://jwglweixin.bupt.edu.cn/api/"
	cases := []struct {
		name       string
		status     int
		body       string
		wantKind   jwErrorKind
		wantErrSub string
	}{
		{
			name:     "auth-code",
			status:   http.StatusOK,
			body:     `{"code":"0","Msg":"token invalid","data":null}`,
			wantKind: jwErrorAuth,
		},
		{
			name:     "http-unauthorized",
			status:   http.StatusUnauthorized,
			body:     `{"code":"0","Msg":"unauthorized"}`,
			wantKind: jwErrorAuth,
		},
		{
			name:     "query-failure",
			status:   http.StatusOK,
			body:     `{"code":"2","Msg":"campus busy","data":null}`,
			wantKind: jwErrorQuery,
		},
		{
			name:     "invalid-json",
			status:   http.StatusOK,
			body:     `{not-json`,
			wantKind: jwErrorParse,
		},
		{
			name:     "invalid-data-shape",
			status:   http.StatusOK,
			body:     `{"code":"1","Msg":"ok","data":{"unexpected":true}}`,
			wantKind: jwErrorParse,
		},
		{
			name:     "null-data-success",
			status:   http.StatusOK,
			body:     `{"code":"1","Msg":"ok","data":null}`,
			wantKind: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doer := serviceHTTPDoerFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: tc.status,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(tc.body)),
				}, nil
			})
			client, err := NewJWClient("u", "p", doer)
			if err != nil {
				t.Fatalf("NewJWClient() error = %v", err)
			}
			rows, err := client.QueryCampus(context.Background(), apiURL, "01", "tok")
			if tc.wantKind == "" {
				if err != nil {
					t.Fatalf("QueryCampus() error = %v, want success", err)
				}
				if len(rows) != 0 {
					t.Fatalf("QueryCampus() rows len = %d, want 0", len(rows))
				}
				return
			}
			if err == nil || !isJWErrorKind(err, tc.wantKind) {
				t.Fatalf("QueryCampus() error = %v, want kind %s", err, tc.wantKind)
			}
		})
	}
}

func TestJWClientQueryCampusHonorsContextCancellation(t *testing.T) {
	blocker := make(chan struct{})
	doer := serviceHTTPDoerFunc(func(req *http.Request) (*http.Response, error) {
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-blocker:
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"code":"1","data":[]}`)),
			}, nil
		}
	})
	client, err := NewJWClient("u", "p", doer)
	if err != nil {
		t.Fatalf("NewJWClient() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := client.QueryCampus(ctx, "https://jwglweixin.bupt.edu.cn/api/", "01", "tok")
		errCh <- err
	}()
	// Allow the request to enter the doer, then cancel.
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("QueryCampus() expected cancellation error")
		}
		if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "context canceled") {
			// Wrapped as jw query request failed; still must surface cancellation.
			if !strings.Contains(strings.ToLower(err.Error()), "cancel") {
				t.Fatalf("QueryCampus() error = %v, want cancellation", err)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("QueryCampus() did not return after cancel")
	}
	close(blocker)
}

func TestJWClientLoginProtocolContract(t *testing.T) {
	expectedPassword, err := encryptJWPassword("fixture-pass")
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}
	var method, path, contentType string
	var form url.Values
	doer := serviceHTTPDoerFunc(func(req *http.Request) (*http.Response, error) {
		method = req.Method
		path = req.URL.Path
		contentType = req.Header.Get("Content-Type")
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		form, err = url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse form: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"code":"1","Msg":"ok","data":{"token":"login-token"}}`)),
		}, nil
	})
	client, err := NewJWClient("fixture-user", "fixture-pass", doer)
	if err != nil {
		t.Fatalf("NewJWClient() error = %v", err)
	}
	token, err := client.Login(context.Background(), "https://jwglweixin.bupt.edu.cn/api/")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if token != "login-token" {
		t.Fatalf("Login() token = %q", token)
	}
	if method != http.MethodPost {
		t.Fatalf("Login method = %q, want POST", method)
	}
	if !strings.HasSuffix(path, "/login") {
		t.Fatalf("Login path = %q, want .../login", path)
	}
	if !strings.Contains(contentType, "application/x-www-form-urlencoded") {
		t.Fatalf("Login Content-Type = %q", contentType)
	}
	if form.Get("userNo") != "fixture-user" || form.Get("pwd") != expectedPassword || form.Get("encode") != "1" {
		t.Fatalf("Login form = %v", form)
	}
}

func TestJWClientFetchAPIURLFallsBackOnInvalidConfig(t *testing.T) {
	doer := serviceHTTPDoerFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != ServerConfigURL {
			t.Fatalf("FetchAPIURL URL = %q, want %q", req.URL.String(), ServerConfigURL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ApiUrl":"http://evil.example/api"}`)),
		}, nil
	})
	client, err := NewJWClient("u", "p", doer)
	if err != nil {
		t.Fatalf("NewJWClient() error = %v", err)
	}
	got, err := client.FetchAPIURL(context.Background())
	if err != nil {
		t.Fatalf("FetchAPIURL() error = %v", err)
	}
	want, err := validateJWAPIURL(DefaultAPIURL)
	if err != nil {
		t.Fatalf("validate default: %v", err)
	}
	if got != want {
		t.Fatalf("FetchAPIURL() = %q, want default %q", got, want)
	}
}
