package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"BUPT_EC/service/model"
	"BUPT_EC/utils"
)

// JWClient is the stateless protocol-level client for the JW system.
// Token and API URL caching live in TokenManager, not here.
type JWClient interface {
	QueryCampus(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error)
	Login(ctx context.Context, apiURL string) (string, error)
	FetchAPIURL(ctx context.Context) (string, error)
}

type defaultJWClient struct {
	username string
	password string
	client   utils.HTTPDoer
}

func NewJWClient(username, password string, client utils.HTTPDoer) (JWClient, error) {
	if isNilDependency(client) {
		return nil, errors.New("JW HTTP client is required")
	}
	return &defaultJWClient{
		username: username,
		password: password,
		client:   client,
	}, nil
}

func (c *defaultJWClient) QueryCampus(ctx context.Context, apiURL string, campusID string, token string) ([]model.JWClassInfo, error) {
	requestCtx, cancel := context.WithTimeout(ctx, jwRequestTimeout)
	defer cancel()

	queryURL, err := joinAPIPath(apiURL, "todayClassrooms")
	if err != nil {
		return nil, newJWError(jwErrorConfig, "jw query", err, "build query URL failed")
	}
	queryURL = addQuery(queryURL, map[string]string{"campusId": campusID})

	code, _, body, err := utils.HttpPostWithHeader(c.client, requestCtx, queryURL, map[string]string{"token": token})
	if err != nil {
		return nil, newJWError(jwErrorQuery, "jw query", err, "request failed")
	}
	if code != 200 {
		kind, message := classifyJWHTTPError(code, body)
		return nil, newJWError(kind, "jw query", nil, "%s", message)
	}

	return parseJWQueryResponse(body)
}

func (c *defaultJWClient) Login(ctx context.Context, apiURL string) (string, error) {
	requestCtx, cancel := context.WithTimeout(ctx, jwRequestTimeout)
	defer cancel()

	loginURL, err := joinAPIPath(apiURL, "login")
	if err != nil {
		return "", newJWError(jwErrorConfig, "jw login", err, "build login URL failed")
	}

	userNo := c.username
	password := c.password
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

	code, _, body, err := utils.HttpPostForm(c.client, requestCtx, loginURL, req)
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

func (c *defaultJWClient) FetchAPIURL(ctx context.Context) (string, error) {
	requestCtx, cancel := context.WithTimeout(ctx, jwRequestTimeout)
	defer cancel()

	code, _, body, err := utils.HttpGet(c.client, requestCtx, ServerConfigURL)
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

type jwResponseEnvelope struct {
	Code string          `json:"code"`
	Msg  string          `json:"Msg"`
	Data json.RawMessage `json:"data"`
}

func parseJWQueryResponse(body []byte) ([]model.JWClassInfo, error) {
	var envelope jwResponseEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, newJWError(jwErrorParse, "jw query", err, "response invalid")
	}
	if envelope.Code != "1" {
		kind := jwErrorQuery
		if isAuthFailureCode(envelope.Code) || isAuthFailureMessage(envelope.Msg) {
			kind = jwErrorAuth
		}
		return nil, newJWError(kind, "jw query", nil, "%s", safeRemoteMessage(envelope.Msg))
	}

	var rows []model.JWClassInfo
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return rows, nil
	}
	if err := json.Unmarshal(envelope.Data, &rows); err != nil {
		return nil, newJWError(jwErrorParse, "jw query", err, "response data invalid")
	}
	return rows, nil
}

func classifyJWHTTPError(statusCode int, body []byte) (jwErrorKind, string) {
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return jwErrorAuth, fmt.Sprintf("http status %d", statusCode)
	}

	var envelope jwResponseEnvelope
	if len(body) > 0 && json.Unmarshal(body, &envelope) == nil {
		message := safeRemoteMessage(envelope.Msg)
		if isAuthFailureCode(envelope.Code) || isAuthFailureMessage(envelope.Msg) {
			return jwErrorAuth, message
		}
		if message != "" && message != "remote service returned failure" {
			return jwErrorQuery, fmt.Sprintf("http status %d: %s", statusCode, message)
		}
	}

	return jwErrorQuery, fmt.Sprintf("http status %d", statusCode)
}
