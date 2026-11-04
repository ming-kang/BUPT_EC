package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDotenvAndEnvironmentPrecedence(t *testing.T) {
	tests := []struct {
		name        string
		dotenv      string
		environment map[string]string
		want        RuntimeConfig
	}{
		{
			name:        "missing dotenv uses environment and defaults",
			environment: map[string]string{"JW_USERNAME": "user", "JW_PASSWORD": "pass"},
			want:        RuntimeConfig{JWUsername: "user", JWPassword: "pass", AppAddr: DefaultAppAddr},
		},
		{
			name: "dotenv values are loaded",
			dotenv: strings.Join([]string{
				"JW_USERNAME=dotenv-user",
				"JW_PASSWORD=dotenv-password",
				"APP_ADDR=localhost:9090",
				"LOG_CALLER=TrUe",
			}, "\n"),
			want: RuntimeConfig{JWUsername: "dotenv-user", JWPassword: "dotenv-password", AppAddr: "localhost:9090", LogCaller: true},
		},
		{
			name: "environment wins over dotenv including explicit empty values",
			dotenv: strings.Join([]string{
				"JW_USERNAME=dotenv-user",
				"JW_PASSWORD=dotenv-password",
				"APP_ADDR=localhost:9090",
				"LOG_CALLER=true",
			}, "\n"),
			environment: map[string]string{"JW_USERNAME": "env-user", "APP_ADDR": "", "LOG_CALLER": "0"},
			want:        RuntimeConfig{JWUsername: "env-user", JWPassword: "dotenv-password", AppAddr: DefaultAppAddr},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dotenvPath := filepath.Join(t.TempDir(), ".env")
			if tt.dotenv != "" {
				if err := os.WriteFile(dotenvPath, []byte(tt.dotenv), 0600); err != nil {
					t.Fatalf("write dotenv fixture: %v", err)
				}
			}
			got, err := Load(dotenvPath, mapLookup(tt.environment))
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Load() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestLoadRejectsBadDotenvWithoutLeakingContents(t *testing.T) {
	malformed := filepath.Join(t.TempDir(), ".env")
	secret := "secret-value-must-not-appear"
	if err := os.WriteFile(malformed, []byte("JW_PASSWORD='"+secret), 0600); err != nil {
		t.Fatalf("write malformed dotenv: %v", err)
	}
	for name, path := range map[string]string{"malformed": malformed, "unreadable": t.TempDir()} {
		_, err := Load(path, mapLookup(nil))
		if err == nil || err.Error() != "load .env file failed" {
			t.Fatalf("%s: Load() error = %v, want safe dotenv error", name, err)
		}
	}
}

func TestLoadValidation(t *testing.T) {
	valid := map[string]string{"JW_USERNAME": "user", "JW_PASSWORD": "password"}
	with := func(key, value string) map[string]string {
		values := map[string]string{key: value}
		for k, v := range valid {
			if k != key {
				values[k] = v
			}
		}
		return values
	}
	tests := []struct {
		name    string
		values  map[string]string
		wantErr bool
	}{
		{name: "valid", values: valid},
		{name: "loopback", values: with("APP_ADDR", "127.0.0.1:8080")},
		{name: "all interfaces", values: with("APP_ADDR", ":8080")},
		{name: "bracketed IPv6", values: with("APP_ADDR", "[::1]:8080")},
		{name: "username missing", values: with("JW_USERNAME", ""), wantErr: true},
		{name: "password missing", values: with("JW_PASSWORD", ""), wantErr: true},
		{name: "missing port", values: with("APP_ADDR", "localhost"), wantErr: true},
		{name: "non numeric port", values: with("APP_ADDR", "localhost:http"), wantErr: true},
		{name: "port zero", values: with("APP_ADDR", "localhost:0"), wantErr: true},
		{name: "port too large", values: with("APP_ADDR", "localhost:65536"), wantErr: true},
		{name: "URL is not listen address", values: with("APP_ADDR", "http://localhost:8080"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(filepath.Join(t.TempDir(), "missing.env"), mapLookup(tt.values))
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func mapLookup(values map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
