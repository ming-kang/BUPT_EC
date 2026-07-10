package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"time"
)

const maxResponseBodyBytes int64 = 5 << 20

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// CheckRedirect rejects every redirect. JW traffic carries a custom token
// header and login form bodies that must never be forwarded to an
// unvalidated host; net/http would otherwise follow redirects and preserve
// those credentials even across hosts.
func CheckRedirect(req *http.Request, via []*http.Request) error {
	return fmt.Errorf("refusing redirect to %q: JW outbound HTTP does not follow redirects", req.URL.String())
}

func NewHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 12 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		CheckRedirect: CheckRedirect,
	}
}

func HttpGet(client HTTPDoer, ctx context.Context, rawURL string) (int, http.Header, []byte, error) {
	return httpRequest(client, ctx, http.MethodGet, rawURL, nil, nil, nil)
}

func HttpPostForm(client HTTPDoer, ctx context.Context, rawURL string, data map[string]string) (int, http.Header, []byte, error) {
	values := url.Values{}
	for key, value := range data {
		values.Set(key, value)
	}
	headers := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	return httpRequest(client, ctx, http.MethodPost, rawURL, headers, nil, []byte(values.Encode()))
}

func HttpPostWithHeader(client HTTPDoer, ctx context.Context, rawURL string, headers map[string]string) (int, http.Header, []byte, error) {
	return httpRequest(client, ctx, http.MethodPost, rawURL, headers, nil, nil)
}

func httpRequest(client HTTPDoer, ctx context.Context, method string, rawURL string, headers map[string]string, query map[string]string, body []byte) (int, http.Header, []byte, error) {
	if isNilHTTPDoer(client) {
		return 0, nil, nil, errors.New("HTTP client is required")
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return 0, nil, nil, err
	}
	if len(query) > 0 {
		q := parsedURL.Query()
		for key, value := range query {
			q.Set(key, value)
		}
		parsedURL.RawQuery = q.Encode()
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewBuffer(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, parsedURL.String(), reader)
	if err != nil {
		return 0, nil, nil, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json, text/plain, */*")
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()

	limitedBody := io.LimitReader(resp.Body, maxResponseBodyBytes+1)
	respBody, err := io.ReadAll(limitedBody)
	if err != nil {
		return resp.StatusCode, resp.Header, nil, err
	}
	if int64(len(respBody)) > maxResponseBodyBytes {
		return resp.StatusCode, resp.Header, nil, fmt.Errorf("http response body exceeds %d bytes", maxResponseBodyBytes)
	}
	return resp.StatusCode, resp.Header, respBody, nil
}

func isNilHTTPDoer(client HTTPDoer) bool {
	if client == nil {
		return true
	}
	value := reflect.ValueOf(client)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
