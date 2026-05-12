package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	envPort        = "PORT"
	envLogLevel    = "LOG_LEVEL"
	envDataDir     = "DATA_DIR"
	envDatabaseURL = "DATABASE_URL"
	envConfigPath  = "CONFIG_PATH"

	defaultPort       = "8080"
	defaultLogLevel   = "info"
	defaultDataDir    = "./data"
	defaultConfigPath = "./configs/config.yaml"
)

var (
	errDatabaseURLRequired   = errors.New("DATABASE_URL is required")
	errReaperDurationInvalid = errors.New("reaper.timeout and reaper.tick must be positive")
)

type Config struct {
	Port        string
	LogLevel    string
	DataDir     string
	DatabaseURL string
	Reaper      ReaperConfig
}

type ReaperConfig struct {
	Timeout time.Duration
	Tick    time.Duration
}

type fileConfig struct {
	Reaper struct {
		Timeout string `yaml:"timeout"`
		Tick    string `yaml:"tick"`
	} `yaml:"reaper"`
}

func Load() (Config, error) {
	cfg := Config{
		Port:        envOr(envPort, defaultPort),
		LogLevel:    envOr(envLogLevel, defaultLogLevel),
		DataDir:     envOr(envDataDir, defaultDataDir),
		DatabaseURL: os.Getenv(envDatabaseURL),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errDatabaseURLRequired
	}

	configPath := envOr(envConfigPath, defaultConfigPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", configPath, err)
	}

	var fc fileConfig

	if uErr := yaml.Unmarshal(data, &fc); uErr != nil {
		return Config{}, fmt.Errorf("parse config: %w", uErr)
	}

	cfg.Reaper.Timeout, err = time.ParseDuration(fc.Reaper.Timeout)
	if err != nil {
		return Config{}, fmt.Errorf("parse reaper.timeout: %w", err)
	}

	cfg.Reaper.Tick, err = time.ParseDuration(fc.Reaper.Tick)
	if err != nil {
		return Config{}, fmt.Errorf("parse reaper.tick: %w", err)
	}

	if cfg.Reaper.Timeout <= 0 || cfg.Reaper.Tick <= 0 {
		return Config{}, errReaperDurationInvalid
	}

	return cfg, nil
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}

	return def
}
