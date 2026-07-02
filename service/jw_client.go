package service

import (
	"BUPT_EC/service/model"
	"BUPT_EC/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type jwResponseEnvelope struct {
	Code string          `json:"code"`
	Msg  string          `json:"Msg"`
	Data json.RawMessage `json:"data"`
}

func queryCampus(ctx context.Context, campusID string, forceRefresh bool) ([]model.JWClassInfo, error) {
	token, err := tokenManager.EnsureToken(ctx, forceRefresh)
	if err != nil {
		return nil, err
	}

	rows, err := queryCampusWithTokenFunc(ctx, campusID, token)
	if err == nil {
		return rows, nil
	}
	if !isJWErrorKind(err, jwErrorAuth) {
		return nil, err
	}

	tokenManager.clearTokenIfCurrent(token)
	token, refreshErr := tokenManager.EnsureToken(ctx, true)
	if refreshErr != nil {
		return nil, errors.Join(err, refreshErr)
	}
	rows, retryErr := queryCampusWithTokenFunc(ctx, campusID, token)
	if retryErr != nil {
		return nil, errors.Join(err, retryErr)
	}
	return rows, nil
}

func queryCampusWithToken(ctx context.Context, campusID string, token string) ([]model.JWClassInfo, error) {
	requestCtx, cancel := context.WithTimeout(ctx, jwRequestTimeout)
	defer cancel()

	apiURL, err := tokenManager.APIURL(requestCtx)
	if err != nil {
		return nil, err
	}
	queryURL, err := joinAPIPath(apiURL, "todayClassrooms")
	if err != nil {
		return nil, newJWError(jwErrorConfig, "jw query", err, "build query URL failed")
	}
	queryURL = addQuery(queryURL, map[string]string{"campusId": campusID})

	code, _, body, err := utils.HttpPostWithHeader(requestCtx, queryURL, map[string]string{"token": token})
	if err != nil {
		return nil, newJWError(jwErrorQuery, "jw query", err, "request failed")
	}
	if code != 200 {
		kind, message := classifyJWHTTPError(code, body)
		return nil, newJWError(kind, "jw query", nil, "%s", message)
	}

	return parseJWQueryResponse(body)
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
