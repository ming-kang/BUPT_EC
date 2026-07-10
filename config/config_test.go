package config

import (
	"os"
	"path/filepath"
	"reflect"
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
			environment: map[string]string{JWTokenKey: "runtime-token"},
			want: RuntimeConfig{
				JW:        JWCredentials{Token: "runtime-token"},
				AppAddr:   DefaultAppAddr,
				GinMode:   DefaultGinMode,
				LogCaller: false,
				Campuses:  defaultCampusesForTest(),
			},
		},
		{
			name: "valid dotenv is loaded without mutating process environment",
			dotenv: strings.Join([]string{
				"JW_USERNAME=dotenv-user",
				"JW_PASSWORD=dotenv-password",
				"APP_ADDR=localhost:9090",
				"GIN_MODE=release",
				"LOG_CALLER=TrUe",
			}, "\n"),
			want: RuntimeConfig{
				JW:        JWCredentials{Username: "dotenv-user", Password: "dotenv-password"},
				AppAddr:   "localhost:9090",
				GinMode:   "release",
				LogCaller: true,
				Campuses:  defaultCampusesForTest(),
			},
		},
		{
			name: "environment wins over dotenv including explicit empty defaults",
			dotenv: strings.Join([]string{
				"JW_TOKEN=dotenv-token",
				"APP_ADDR=localhost:9090",
				"GIN_MODE=release",
				"LOG_CALLER=true",
			}, "\n"),
			environment: map[string]string{
				JWTokenKey:   "runtime-token",
				AppAddrKey:   "",
				GinModeKey:   "test",
				LogCallerKey: "0",
			},
			want: RuntimeConfig{
				JW:        JWCredentials{Token: "runtime-token"},
				AppAddr:   DefaultAppAddr,
				GinMode:   "test",
				LogCaller: false,
				Campuses:  defaultCampusesForTest(),
			},
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
			before, existed := os.LookupEnv(JWTokenKey)
			got, err := Load(dotenvPath, mapLookup(tt.environment))
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatal("Load() returned an unexpected runtime configuration")
			}
			after, stillExists := os.LookupEnv(JWTokenKey)
			if existed != stillExists || before != after {
				t.Fatal("Load() mutated the process environment")
			}
		})
	}
}

func TestLoadRejectsPresentInvalidDotenvWithoutLeakingContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	secret := "secret-value-must-not-appear"
	if err := os.WriteFile(path, []byte("JW_TOKEN='"+secret), 0600); err != nil {
		t.Fatalf("write malformed dotenv: %v", err)
	}

	_, err := Load(path, mapLookup(nil))
	if err == nil {
		t.Fatal("Load() expected malformed dotenv error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("Load() error leaked dotenv contents: %v", err)
	}
}

func TestLoadRejectsUnreadableDotenv(t *testing.T) {
	_, err := Load(t.TempDir(), mapLookup(map[string]string{JWTokenKey: "token"}))
	if err == nil {
		t.Fatal("Load() expected unreadable dotenv error")
	}
	if err.Error() != "load .env file failed" {
		t.Fatalf("Load() error = %q, want safe dotenv error", err)
	}
}

func TestLoadCredentialValidation(t *testing.T) {
	tests := []struct {
		name    string
		values  map[string]string
		wantErr bool
	}{
		{name: "token only", values: map[string]string{JWTokenKey: "token"}},
		{name: "username and password", values: map[string]string{JWUsernameKey: "user", JWPasswordKey: "password"}},
		{name: "token permits incomplete login pair", values: map[string]string{JWTokenKey: "token", JWUsernameKey: "user"}},
		{name: "username only", values: map[string]string{JWUsernameKey: "user"}, wantErr: true},
		{name: "password only", values: map[string]string{JWPasswordKey: "password"}, wantErr: true},
		{name: "empty", values: map[string]string{}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(filepath.Join(t.TempDir(), "missing.env"), mapLookup(tt.values))
			if tt.wantErr {
				if err == nil {
					t.Fatal("Load() expected credential error")
				}
				for _, value := range tt.values {
					if value != "" && strings.Contains(err.Error(), value) {
						t.Fatalf("Load() error leaked credential value: %v", err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if !cfg.HasJWCredentials() {
				t.Fatal("HasJWCredentials() = false, want true")
			}
		})
	}
}

func TestLoadAppAddrValidation(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		want    string
		wantErr bool
	}{
		{name: "default", want: DefaultAppAddr},
		{name: "loopback", addr: "127.0.0.1:8080", want: "127.0.0.1:8080"},
		{name: "all interfaces", addr: ":8080", want: ":8080"},
		{name: "hostname", addr: "localhost:9090", want: "localhost:9090"},
		{name: "bracketed IPv6", addr: "[::1]:8080", want: "[::1]:8080"},
		{name: "missing port", addr: "localhost", wantErr: true},
		{name: "non numeric port", addr: "localhost:http", wantErr: true},
		{name: "port zero", addr: "localhost:0", wantErr: true},
		{name: "port too large", addr: "localhost:65536", wantErr: true},
		{name: "URL is not listen address", addr: "http://localhost:8080", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := map[string]string{JWTokenKey: "token", AppAddrKey: tt.addr}
			cfg, err := Load(filepath.Join(t.TempDir(), "missing.env"), mapLookup(values))
			if tt.wantErr {
				if err == nil {
					t.Fatal("Load() expected APP_ADDR error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.AppAddr != tt.want {
				t.Fatalf("AppAddr = %q, want %q", cfg.AppAddr, tt.want)
			}
		})
	}
}

func TestLoadGinModeAndLogCaller(t *testing.T) {
	tests := []struct {
		name          string
		ginMode       string
		logCaller     string
		wantMode      string
		wantLogCaller bool
		wantErr       bool
	}{
		{name: "defaults", wantMode: DefaultGinMode},
		{name: "debug and one", ginMode: "debug", logCaller: "1", wantMode: "debug", wantLogCaller: true},
		{name: "release and mixed true", ginMode: "release", logCaller: "TrUe", wantMode: "release", wantLogCaller: true},
		{name: "test and false", ginMode: "test", logCaller: "false", wantMode: "test"},
		{name: "invalid mode", ginMode: "production", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(filepath.Join(t.TempDir(), "missing.env"), mapLookup(map[string]string{
				JWTokenKey:   "token",
				GinModeKey:   tt.ginMode,
				LogCallerKey: tt.logCaller,
			}))
			if tt.wantErr {
				if err == nil {
					t.Fatal("Load() expected GIN_MODE error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.GinMode != tt.wantMode || cfg.LogCaller != tt.wantLogCaller {
				t.Fatalf("mode/log caller = %q/%v, want %q/%v", cfg.GinMode, cfg.LogCaller, tt.wantMode, tt.wantLogCaller)
			}
		})
	}
}

func TestLoadReturnsIndependentCampusSlices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.env")
	first, err := Load(path, mapLookup(map[string]string{JWTokenKey: "token"}))
	if err != nil {
		t.Fatalf("first Load() error = %v", err)
	}
	first.Campuses[0].Name = "changed"
	second, err := Load(path, mapLookup(map[string]string{JWTokenKey: "token"}))
	if err != nil {
		t.Fatalf("second Load() error = %v", err)
	}
	if !reflect.DeepEqual(second.Campuses, defaultCampusesForTest()) {
		t.Fatalf("second campuses = %#v, want fixed defaults", second.Campuses)
	}
}

func TestLoadRequiresLookup(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.env"), nil)
	if err == nil {
		t.Fatal("Load() expected missing lookup error")
	}
}

func mapLookup(values map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func defaultCampusesForTest() []CampusConfig {
	return []CampusConfig{
		{ID: "01", Name: "西土城"},
		{ID: "04", Name: "沙河"},
	}
}
