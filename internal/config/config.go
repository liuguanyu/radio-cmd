package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Schedule represents a scheduled playback
type Schedule struct {
	ID          string `json:"id"`
	StationID   string `json:"station_id"`
	StationName string `json:"station_name"`
	// Time format: "15:04" for daily, "2006-01-02T15:04" for one-time
	PlayTime    string `json:"play_time"`
	Recurring   bool   `json:"recurring"` // true for daily, false for one-time
	Enabled     bool   `json:"enabled"`
	CreatedAt   int64  `json:"created_at"`
}

// Config represents the application configuration
type Config struct {
	DefaultProvince  string     `json:"default_province"`
	UseHighQuality   bool       `json:"use_high_quality"`
	AutoRefresh      bool       `json:"auto_refresh"`
	RefreshInterval  int        `json:"refresh_interval_minutes"` // 0 = disabled
	MaxFailedRetries int        `json:"max_failed_retries"`
	CacheStations    bool       `json:"cache_stations"`
	CacheTTLMinutes  int        `json:"cache_ttl_minutes"`
	Schedules        []Schedule `json:"schedules"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultProvince:  "0",
		UseHighQuality:   true,
		AutoRefresh:      false,
		RefreshInterval:  30,
		MaxFailedRetries: 3,
		CacheStations:    true,
		CacheTTLMinutes:  60,
		Schedules:        []Schedule{},
	}
}

// LoadConfig loads configuration from file
func LoadConfig() (*Config, error) {
	configDir := filepath.Join(os.Getenv("HOME"), ".radio-cmd")
	configFile := filepath.Join(configDir, "config.json")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}

	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// Create default config file
		return DefaultConfig(), SaveConfig(DefaultConfig())
	}

	// Read existing config file
	file, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveConfig saves configuration to file
func SaveConfig(config *Config) error {
	configDir := filepath.Join(os.Getenv("HOME"), ".radio-cmd")
	configFile := filepath.Join(configDir, "config.json")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	file, err := os.Create(configFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(config)
}
