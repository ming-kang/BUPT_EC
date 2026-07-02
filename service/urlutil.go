package service

import (
	"net/url"
	"strings"
)

func joinAPIPath(apiURL string, path string) (string, error) {
	parsed, err := url.Parse(apiURL)
	if err != nil {
		return "", err
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + strings.TrimLeft(path, "/")
	return parsed.String(), nil
}

func addQuery(rawURL string, values map[string]string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	for key, value := range values {
		query.Set(key, value)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func validateJWAPIURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", newJWError(jwErrorConfig, "serverconfig", err, "invalid API URL")
	}
	if parsed.Scheme != "https" {
		return "", newJWError(jwErrorConfig, "serverconfig", nil, "API URL must use HTTPS")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", newJWError(jwErrorConfig, "serverconfig", nil, "API URL host is empty")
	}
	if host != "jwglweixin.bupt.edu.cn" && !strings.HasSuffix(host, ".bupt.edu.cn") {
		return "", newJWError(jwErrorConfig, "serverconfig", nil, "API URL host is not allowed")
	}
	if parsed.User != nil {
		return "", newJWError(jwErrorConfig, "serverconfig", nil, "API URL must not contain user info")
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return "", newJWError(jwErrorConfig, "serverconfig", nil, "API URL port is not allowed")
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}
