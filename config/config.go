// Package config loads the immutable startup configuration. Process
// environment values take precedence over an optional .env file.
package config

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const DefaultAppAddr = "127.0.0.1:8080"

var errInvalidAppAddr = errors.New("APP_ADDR must be a valid host:port with port 1-65535")

type RuntimeConfig struct {
	JWUsername string
	JWPassword string
	AppAddr    string
	LogCaller  bool
}

type LookupEnv func(string) (string, bool)

func Load(dotenvPath string, lookup LookupEnv) (RuntimeConfig, error) {
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

	logCaller := resolve("LOG_CALLER")
	cfg := RuntimeConfig{
		JWUsername: resolve("JW_USERNAME"),
		JWPassword: resolve("JW_PASSWORD"),
		AppAddr:    resolve("APP_ADDR"),
		LogCaller:  logCaller == "1" || strings.EqualFold(logCaller, "true"),
	}
	if cfg.AppAddr == "" {
		cfg.AppAddr = DefaultAppAddr
	}
	if cfg.JWUsername == "" || cfg.JWPassword == "" {
		return RuntimeConfig{}, errors.New("JW_USERNAME and JW_PASSWORD are required")
	}
	if err := validateAppAddr(cfg.AppAddr); err != nil {
		return RuntimeConfig{}, err
	}
	return cfg, nil
}

func readDotenv(path string) (map[string]string, error) {
	values, err := godotenv.Read(path)
	if err == nil {
		return values, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	// Never echo the parser error: it may quote file contents.
	return nil, errors.New("load .env file failed")
}

func validateAppAddr(addr string) error {
	if strings.ContainsAny(addr, "/; \t\r\n") {
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
