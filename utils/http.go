package utils

import (
	"BUPT_EC/logs"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const maxResponseBodyBytes int64 = 5 << 20

var defaultHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

func HttpGet(ctx context.Context, rawURL string) (int, http.Header, []byte, error) {
	return httpRequest(ctx, "GET", rawURL, nil, nil, nil)
}

func HttpPostForm(ctx context.Context, rawURL string, data map[string]string) (int, http.Header, []byte, error) {
	values := url.Values{}
	for key, value := range data {
		values.Set(key, value)
	}
	headers := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	return httpRequest(ctx, "POST", rawURL, headers, nil, []byte(values.Encode()))
}

func HttpPostWithHeader(ctx context.Context, rawURL string, headers map[string]string) (int, http.Header, []byte, error) {
	return httpRequest(ctx, "POST", rawURL, headers, nil, nil)
}

func httpRequest(ctx context.Context, method string, rawURL string, headers map[string]string, query map[string]string, body []byte) (int, http.Header, []byte, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		logs.CtxError(ctx, "http url parse error: %v", err)
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
		logs.CtxError(ctx, "http request create error: %v", err)
		return 0, nil, nil, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json, text/plain, */*")
	}

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		logs.CtxError(ctx, "http request error: %v", err)
		return 0, nil, nil, err
	}
	defer resp.Body.Close()

	limitedBody := io.LimitReader(resp.Body, maxResponseBodyBytes+1)
	respBody, err := io.ReadAll(limitedBody)
	if err != nil {
		logs.CtxError(ctx, "http response read error: %v", err)
		return resp.StatusCode, resp.Header, nil, err
	}
	if int64(len(respBody)) > maxResponseBodyBytes {
		return resp.StatusCode, resp.Header, nil, fmt.Errorf("http response body exceeds %d bytes", maxResponseBodyBytes)
	}
	return resp.StatusCode, resp.Header, respBody, nil
}
