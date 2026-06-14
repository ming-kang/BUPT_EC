package config

import (
	"errors"
	"os"
	"sync"

	"github.com/joho/godotenv"
)

type CampusConfig struct {
	ID   string
	Name string
}

type Config struct {
	Campuses []CampusConfig
}

var (
	GlobalConfig *Config
	initOnce     sync.Once
)

func InitConfig() {
	initOnce.Do(func() {
		_ = godotenv.Load()
		GlobalConfig = &Config{
			Campuses: []CampusConfig{
				{ID: "01", Name: "西土城"},
				{ID: "04", Name: "沙河"},
			},
		}
	})
}

func GetConfig() Config {
	InitConfig()
	return *GlobalConfig
}

func HasJWCredentials() bool {
	InitConfig()
	if os.Getenv("JW_TOKEN") != "" {
		return true
	}
	return os.Getenv("JW_USERNAME") != "" && os.Getenv("JW_PASSWORD") != ""
}

func ValidateRuntimeConfig() error {
	InitConfig()
	if !HasJWCredentials() {
		return errors.New("JW_TOKEN or JW_USERNAME/JW_PASSWORD is required")
	}
	return nil
}
