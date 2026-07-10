package config

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const (
	JWUsernameKey = "JW_USERNAME"
	JWPasswordKey = "JW_PASSWORD"
	JWTokenKey    = "JW_TOKEN"
	AppAddrKey    = "APP_ADDR"
	GinModeKey    = "GIN_MODE"
	LogCallerKey  = "LOG_CALLER"

	DefaultAppAddr = "127.0.0.1:8080"
	DefaultGinMode = "debug"
)

var errInvalidAppAddr = errors.New("APP_ADDR must be a valid host:port with port 1-65535")

type CampusConfig struct {
	ID   string
	Name string
}

type JWCredentials struct {
	Username string
	Password string
	Token    string
}

type RuntimeConfig struct {
	JW        JWCredentials
	AppAddr   string
	GinMode   string
	LogCaller bool
	Campuses  []CampusConfig
}

type LookupEnv func(string) (string, bool)

func Load(dotenvPath string, lookup LookupEnv) (RuntimeConfig, error) {
	if lookup == nil {
		return RuntimeConfig{}, errors.New("environment lookup is required")
	}

	dotenv, err := readDotenv(dotenvPath)
	if err != nil {
		return RuntimeConfig{}, err
	}
	resolve := func(key string) string {
		if value, ok := lookup(key); ok {
			return value
		}
		return dotenv[key]
	}

	cfg := RuntimeConfig{
		JW: JWCredentials{
			Username: resolve(JWUsernameKey),
			Password: resolve(JWPasswordKey),
			Token:    resolve(JWTokenKey),
		},
		AppAddr:   resolve(AppAddrKey),
		GinMode:   resolve(GinModeKey),
		LogCaller: parseLogCaller(resolve(LogCallerKey)),
		Campuses: []CampusConfig{
			{ID: "01", Name: "西土城"},
			{ID: "04", Name: "沙河"},
		},
	}
	if cfg.AppAddr == "" {
		cfg.AppAddr = DefaultAppAddr
	}
	if cfg.GinMode == "" {
		cfg.GinMode = DefaultGinMode
	}
	if err := cfg.validate(); err != nil {
		return RuntimeConfig{}, err
	}
	return cfg, nil
}

func (c RuntimeConfig) HasJWCredentials() bool {
	return c.JW.Token != "" || (c.JW.Username != "" && c.JW.Password != "")
}

func (c RuntimeConfig) validate() error {
	if !c.HasJWCredentials() {
		return errors.New("JW_TOKEN or JW_USERNAME/JW_PASSWORD is required")
	}
	if c.GinMode != "debug" && c.GinMode != "release" && c.GinMode != "test" {
		return errors.New("GIN_MODE must be debug, release, or test")
	}
	if err := validateAppAddr(c.AppAddr); err != nil {
		return err
	}
	return nil
}

func readDotenv(path string) (map[string]string, error) {
	values, err := godotenv.Read(path)
	if err == nil {
		return values, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	return nil, errors.New("load .env file failed")
}

func parseLogCaller(value string) bool {
	return value == "1" || strings.EqualFold(value, "true")
}

func validateAppAddr(addr string) error {
	if addr == "" || strings.ContainsAny(addr, "/; \t\r\n") {
		return errInvalidAppAddr
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return errInvalidAppAddr
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return errInvalidAppAddr
	}
	return nil
}
