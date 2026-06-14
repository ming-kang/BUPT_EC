package utils

import (
	"EmptyClassroom/logs"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
)

func HttpGet(ctx context.Context, rawURL string) (int, http.Header, []byte, error) {
	return httpRequest(ctx, "GET", rawURL, nil, nil, nil)
}

func HttpPostForm(ctx context.Context, rawURL string, data map[string]string) (int, http.Header, []byte, error) {
	return httpRequest(ctx, "POST", rawURL, nil, data, nil)
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

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logs.CtxError(ctx, "http request error: %v", err)
		return 0, nil, nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, respBody, nil
}
