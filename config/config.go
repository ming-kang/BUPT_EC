package config

import (
	"github.com/joho/godotenv"
)

type CampusConfig struct {
	ID   string
	Name string
}

type Config struct {
	Campuses []CampusConfig
}

var GlobalConfig *Config

func InitConfig() {
	_ = godotenv.Load()
	GlobalConfig = &Config{
		Campuses: []CampusConfig{
			{ID: "01", Name: "西土城"},
			{ID: "04", Name: "沙河"},
		},
	}
}

func GetConfig() Config {
	if GlobalConfig == nil {
		InitConfig()
	}
	return *GlobalConfig
}
