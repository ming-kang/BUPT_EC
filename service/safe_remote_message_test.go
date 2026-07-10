package service

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSafeRemoteMessageSanitizesAndBounds(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: "remote service returned failure"},
		{name: "whitespace only", in: " \n\t ", want: "remote service returned failure"},
		{name: "control chars", in: "line1\r\nline2\tstatus", want: "line1 line2 status"},
		{name: "token kv", in: "token=abc123 still ok", want: "token=[REDACTED] still ok"},
		{name: "password colon", in: "password: super-secret", want: "password=[REDACTED]"},
		{name: "bearer", in: "Bearer FAKESECRET_y4z5a6b7c8d9e0f1g2h3", want: "Bearer [REDACTED]"},
		{name: "chinese account", in: "账号：2020123456 查询失败", want: "账号=[REDACTED] 查询失败"},
		{name: "ordinary chinese", in: "活动已过期", want: "活动已过期"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := safeRemoteMessage(tc.in)
			if got != tc.want {
				t.Fatalf("safeRemoteMessage(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if strings.Contains(got, "\n") || strings.Contains(got, "\r") {
				t.Fatalf("safeRemoteMessage retained control characters: %q", got)
			}
		})
	}
}

func TestSafeRemoteMessageTruncatesRunes(t *testing.T) {
	in := strings.Repeat("测", safeRemoteMessageRuneLimit+40)
	got := safeRemoteMessage(in)
	if utf8.RuneCountInString(got) != safeRemoteMessageRuneLimit {
		t.Fatalf("rune count = %d, want %d", utf8.RuneCountInString(got), safeRemoteMessageRuneLimit)
	}
	if strings.Contains(got, "token=") {
		t.Fatalf("unexpected redaction in long message: %q", got)
	}
}

func TestSafeErrorMessageStillHidesUpstreamText(t *testing.T) {
	err := newJWError(jwErrorQuery, "jw query", nil, "%s", safeRemoteMessage("token=secret-value campus busy"))
	msg := SafeErrorMessage(err)
	if strings.Contains(msg, "token") || strings.Contains(msg, "secret") {
		t.Fatalf("SafeErrorMessage leaked upstream detail: %q", msg)
	}
}
