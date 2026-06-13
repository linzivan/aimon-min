package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
type Config struct {
	General       GeneralConfig       `yaml:"general"`
	DeepSeek      DeepSeekConfig      `yaml:"deepseek"`
	Notifications NotificationConfig  `yaml:"notifications"`
	Monitor       MonitorConfig       `yaml:"monitor"`
	Proxy         ProxyConfig         `yaml:"proxy"`
	Storage       StorageConfig       `yaml:"storage"`
}

// GeneralConfig holds general application settings.
type GeneralConfig struct {
	AutoStart bool `yaml:"auto_start"`
}

// DeepSeekConfig holds DeepSeek API settings.
type DeepSeekConfig struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
}

// NotificationConfig holds notification thresholds.
type NotificationConfig struct {
	BalanceThreshold    float64 `yaml:"balance_threshold"`
	TokenSurgeThreshold int     `yaml:"token_surge_threshold"`
	CooldownMinutes     int     `yaml:"cooldown_minutes"`
}

// MonitorConfig holds monitoring settings.
type MonitorConfig struct {
	RefreshInterval int `yaml:"refresh_interval"`
}

// ProxyConfig holds reverse proxy settings.
type ProxyConfig struct {
	Enabled bool   `yaml:"enabled"`
	Listen  string `yaml:"listen"`
}

// StorageConfig holds storage settings.
type StorageConfig struct {
	DBPath string `yaml:"db_path"`
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	return &Config{
		General: GeneralConfig{
			AutoStart: false,
		},
		DeepSeek: DeepSeekConfig{
			APIKey:  "",
			BaseURL: "https://api.deepseek.com",
		},
		Notifications: NotificationConfig{
			BalanceThreshold:    10.0,
			TokenSurgeThreshold: 50,
			CooldownMinutes:     30,
		},
		Monitor: MonitorConfig{
			RefreshInterval: 60,
		},
		Proxy: ProxyConfig{
			Enabled: false,
			Listen:  "127.0.0.1:8080",
		},
		Storage: StorageConfig{
			DBPath: "",
		},
	}
}

// Load reads configuration from a YAML file.
// If path is empty, it searches default locations:
//   1. ./config.yaml (current directory)
//   2. %APPDATA%/AI-Monitor/config.yaml
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	paths := []string{}
	if path != "" {
		paths = append(paths, path)
	}
	paths = append(paths, "config.yaml")
	appData := os.Getenv("APPDATA")
	if appData != "" {
		paths = append(paths, filepath.Join(appData, "AI-Monitor", "config.yaml"))
	}

	var loaded bool
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", p, err)
		}
		loaded = true
		fmt.Printf("[config] loaded %s\n", p)
		break
	}

	if !loaded {
		fmt.Println("[config] no config file found, using defaults")
	}
	return cfg, nil
}

// DBPath returns the resolved database path.
func (c *Config) DBPath() string {
	if c.Storage.DBPath != "" {
		return c.Storage.DBPath
	}
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = "."
	}
	dir := filepath.Join(appData, "AI-Monitor")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "monitor.db")
}
