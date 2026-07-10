package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

type jwErrorKind string

const (
	jwErrorAuth     jwErrorKind = "jw_auth_failed"
	jwErrorConfig   jwErrorKind = "jw_config_error"
	jwErrorLogin    jwErrorKind = "jw_login_failed"
	jwErrorQuery    jwErrorKind = "jw_query_failed"
	jwErrorParse    jwErrorKind = "jw_bad_response"
	jwErrorUpstream jwErrorKind = "jw_unavailable"
)

type jwError struct {
	kind jwErrorKind
	op   string
	err  error
	msg  string
}

func (e *jwError) Error() string {
	if e == nil {
		return ""
	}
	if e.err != nil && e.msg != "" {
		return fmt.Sprintf("%s: %s: %v", e.op, e.msg, e.err)
	}
	if e.err != nil {
		return fmt.Sprintf("%s: %v", e.op, e.err)
	}
	if e.msg != "" {
		return fmt.Sprintf("%s: %s", e.op, e.msg)
	}
	return e.op
}

func (e *jwError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func newJWError(kind jwErrorKind, op string, err error, format string, v ...interface{}) error {
	return &jwError{kind: kind, op: op, err: err, msg: fmt.Sprintf(format, v...)}
}

func classifyError(err error) string {
	if err == nil {
		return ""
	}
	var jwErr *jwError
	if errors.As(err, &jwErr) {
		return string(jwErr.kind)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return string(jwErrorUpstream)
	}
	return string(jwErrorUpstream)
}

func SafeErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrNoTodayCache) {
		return "暂无可用的今日空教室数据，请稍后重试"
	}
	switch classifyError(err) {
	case string(jwErrorConfig):
		return "服务配置不完整，请检查教务系统凭据"
	case string(jwErrorAuth), string(jwErrorLogin):
		return "教务系统登录失败，请检查服务配置或稍后重试"
	case string(jwErrorQuery), string(jwErrorParse):
		return "教务系统查询失败，请稍后重试"
	default:
		return "数据获取失败，请稍后重试"
	}
}

const safeRemoteMessageRuneLimit = 256

// safeRemoteMessage normalizes upstream JW text for internal errors and logs.
// Clients never receive this string; they only see SafeErrorMessage.
func safeRemoteMessage(message string) string {
	normalized := sanitizeRemoteMessage(message)
	if normalized == "" {
		return "remote service returned failure"
	}
	return normalized
}

func sanitizeRemoteMessage(message string) string {
	if message == "" {
		return ""
	}

	// Collapse control characters and all whitespace to single spaces so log
	// lines stay single-line and cannot inject fake structured fields.
	var b strings.Builder
	b.Grow(len(message))
	lastSpace := false
	for _, r := range message {
		if r < 0x20 || r == 0x7f {
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
			continue
		}
		if r == ' ' || r == '\u00a0' {
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
			continue
		}
		b.WriteRune(r)
		lastSpace = false
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return ""
	}

	out = redactSensitiveRemoteFragments(out)
	out = truncateRunes(out, safeRemoteMessageRuneLimit)
	return strings.TrimSpace(out)
}

var (
	remoteASCIISecretKV = regexp.MustCompile(`(?i)\b(token|authorization|password|passwd|username|account)\b\s*[:=：]\s*[^\s,;]+`)
	remoteCJKSecretKV   = regexp.MustCompile(`(令牌|密码|账号|学号)\s*[:=：]\s*[^\s,;]+`)
	remoteBearerSecret  = regexp.MustCompile(`(?i)\bbearer\s+[^\s,;]+`)
)

func redactSensitiveRemoteFragments(message string) string {
	out := remoteASCIISecretKV.ReplaceAllStringFunc(message, func(match string) string {
		// Keep the sensitive key label; drop only the value.
		idx := strings.IndexAny(match, ":=：")
		if idx < 0 {
			return "[REDACTED]"
		}
		return strings.TrimSpace(match[:idx]) + "=[REDACTED]"
	})
	out = remoteCJKSecretKV.ReplaceAllStringFunc(out, func(match string) string {
		idx := strings.IndexAny(match, ":=：")
		if idx < 0 {
			return "[REDACTED]"
		}
		return strings.TrimSpace(match[:idx]) + "=[REDACTED]"
	})
	out = remoteBearerSecret.ReplaceAllString(out, "Bearer [REDACTED]")
	return out
}

func truncateRunes(s string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit])
}

func isAuthFailureCode(code string) bool {
	code = strings.TrimSpace(code)
	return code == "401" || code == "403"
}

func isAuthFailureMessage(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	if message == "" {
		return false
	}
	// Avoid bare “过期/失效” — they appear in non-auth JW messages.
	for _, marker := range []string{
		"token", "login", "auth", "unauthorized", "forbidden",
		"登录", "认证", "未授权", "无权限", "重新登录",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func isJWErrorKind(err error, kind jwErrorKind) bool {
	var jwErr *jwError
	if !errors.As(err, &jwErr) {
		return false
	}
	return jwErr.kind == kind
}
