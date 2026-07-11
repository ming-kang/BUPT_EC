package service

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"unicode"
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
		{name: "del", in: "a\x7fb", want: "a b"},
		{name: "c1 next line", in: "line1\u0085line2", want: "line1 line2"},
		{name: "line separator", in: "line1\u2028line2", want: "line1 line2"},
		{name: "paragraph separator", in: "line1\u2029line2", want: "line1 line2"},
		{name: "em space", in: "a\u2003b", want: "a b"},
		{name: "thin space", in: "a\u2009b", want: "a b"},
		{name: "ideographic space", in: "a\u3000b", want: "a b"},
		{name: "zero width space", in: "a\u200bb", want: "a b"},
		{name: "bidi override", in: "a\u202eb", want: "a b"},
		{name: "word joiner", in: "a\u2060b", want: "a b"},
		{name: "mixed unsafe run", in: "a\r\n\u2028\u3000\u200bb", want: "a b"},
		{name: "all unsafe", in: "\u2028\u0085\u3000\u200b", want: "remote service returned failure"},
		{name: "token kv", in: "token=abc123 still ok", want: "token=[REDACTED] still ok"},
		{name: "password colon", in: "password: super-secret", want: "password=[REDACTED]"},
		{name: "bearer", in: "Bearer sk-live-ABCDEFG", want: "Bearer [REDACTED]"},
		{name: "chinese account", in: "账号：2020123456 查询失败", want: "账号=[REDACTED] 查询失败"},
		{name: "ordinary chinese", in: "活动已过期", want: "活动已过期"},
		{name: "emoji preserved", in: "查询失败 😅 请重试", want: "查询失败 😅 请重试"},
		// Unicode whitespace must not bypass redaction (RE2 \s is ASCII-only).
		{name: "token unicode space equals", in: "token\u3000=\u3000secret-value ok", want: "token=[REDACTED] ok"},
		{name: "password unicode space colon", in: "password\u2003:\u2003super-secret", want: "password=[REDACTED]"},
		{name: "account fullwidth colon unicode space", in: "账号\u3000：\u30002020123456 失败", want: "账号=[REDACTED] 失败"},
		{name: "bearer unicode space", in: "Bearer\u3000sk-live-ABCDEFG rest", want: "Bearer [REDACTED] rest"},
		{name: "authorization unicode space", in: "authorization\u0085:\u0085hdr-value", want: "authorization=[REDACTED]"},
		{name: "令牌 line sep", in: "令牌\u2028=\u2028tok-xyz", want: "令牌=[REDACTED]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := safeRemoteMessage(tc.in)
			if got != tc.want {
				t.Fatalf("safeRemoteMessage(%q) = %q, want %q", tc.in, got, tc.want)
			}
			assertSafeRemoteMessageInvariants(t, got)
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
	assertSafeRemoteMessageInvariants(t, got)
}

func TestSafeRemoteMessageRedactsBeforeTruncate(t *testing.T) {
	// Long secret would push past the rune limit if left intact. Redaction must
	// run first so truncation never retains a raw secret tail.
	const secret = "secret-value-that-is-very-long-and-must-not-survive"
	prefix := strings.Repeat("测", safeRemoteMessageRuneLimit-8)
	in := prefix + "token=" + secret
	got := safeRemoteMessage(in)
	if strings.Contains(got, secret) || strings.Contains(got, "secret-value") {
		t.Fatalf("secret survived redaction/truncate: %q", got)
	}
	if !strings.Contains(got, "token=") {
		t.Fatalf("expected redacted key label near limit: %q", got)
	}
	// Full marker may be truncated at the bound; a partial "[RED..." is fine.
	if strings.Contains(got, "token=secret") {
		t.Fatalf("raw secret attached to key after pipeline: %q", got)
	}
	if utf8.RuneCountInString(got) > safeRemoteMessageRuneLimit {
		t.Fatalf("rune count = %d > %d", utf8.RuneCountInString(got), safeRemoteMessageRuneLimit)
	}
	assertSafeRemoteMessageInvariants(t, got)
}

func TestSafeErrorMessageStillHidesUpstreamText(t *testing.T) {
	err := newJWError(jwErrorQuery, "jw query", nil, "%s", safeRemoteMessage("token=secret-value campus busy"))
	msg := SafeErrorMessage(err)
	if strings.Contains(msg, "token") || strings.Contains(msg, "secret") {
		t.Fatalf("SafeErrorMessage leaked upstream detail: %q", msg)
	}
	if msg != "教务系统查询失败，请稍后重试" {
		t.Fatalf("SafeErrorMessage = %q, want fixed query failure text", msg)
	}
}

func TestSafeRemoteMessageSlogJSONSingleLine(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	sanitized := safeRemoteMessage("token\u2028=\u2028secret-value\nsecond-line\u3000ok")
	err := newJWError(jwErrorQuery, "jw query", nil, "%s", sanitized)
	logger.Warn("classroom refresh failed", "err", err)

	out := buf.String()
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("expected exactly one JSON log line, got %q", out)
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &record); err != nil {
		t.Fatalf("log line is not JSON: %v\n%s", err, out)
	}
	errField, _ := record["err"].(string)
	if errField == "" {
		t.Fatalf("missing err field in log: %v", record)
	}
	if strings.Contains(errField, "secret-value") || strings.Contains(errField, "secret") {
		t.Fatalf("secret leaked into slog JSON: %q", errField)
	}
	if strings.ContainsAny(errField, "\r\n") || strings.Contains(errField, "\u2028") || strings.Contains(errField, "\u2029") {
		t.Fatalf("control/line separator in slog err field: %q", errField)
	}
	if !strings.Contains(errField, "[REDACTED]") {
		t.Fatalf("expected redacted token in slog err: %q", errField)
	}
}

func FuzzSafeRemoteMessage(f *testing.F) {
	for _, seed := range []string{
		"",
		" \n\t ",
		"line1\r\nline2",
		"token=abc",
		"Bearer sk-test",
		"账号：123",
		"活动已过期",
		"a\u0085b",
		"a\u2028b",
		"a\u2029b",
		"token\u3000=\u3000secret",
		"Bearer\u2003tok",
		"\u202e\u200b\u3000",
		strings.Repeat("测", 300),
		"password: x" + strings.Repeat("y", 300),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, in string) {
		got := safeRemoteMessage(in)
		assertSafeRemoteMessageInvariants(t, got)
		if strings.Contains(got, "secret-value") {
			// Seed-style secrets must never survive; arbitrary fuzz input may
			// coincidentally contain the substring without a key, which is fine.
		}
	})
}

func assertSafeRemoteMessageInvariants(t *testing.T, got string) {
	t.Helper()
	if got == "" {
		t.Fatalf("safeRemoteMessage returned empty string")
	}
	if utf8.RuneCountInString(got) > safeRemoteMessageRuneLimit {
		t.Fatalf("rune count = %d > limit %d (%q)", utf8.RuneCountInString(got), safeRemoteMessageRuneLimit, got)
	}
	for _, r := range got {
		// ASCII space is the intentional collapse target; every other unsafe
		// class (Unicode spaces, controls, Cf/Zl/Zp) must be gone.
		if r == ' ' {
			continue
		}
		if isUnsafeRemoteRune(r) {
			t.Fatalf("output retains unsafe rune U+%04X in %q", r, got)
		}
		if unicode.IsSpace(r) || unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) {
			t.Fatalf("control/format/separator U+%04X retained in %q", r, got)
		}
	}
	if strings.Contains(got, "\n") || strings.Contains(got, "\r") {
		t.Fatalf("CR/LF retained: %q", got)
	}
}
