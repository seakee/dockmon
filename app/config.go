// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package app defines global configuration models and config loading helpers.
package app

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

const (
	envKey  = "RUN_ENV"
	nameKey = "APP_NAME"
)

// config stores the singleton configuration loaded by LoadConfig.
var config *Config

type (
	// Config is the root configuration model loaded from bin/configs/*.json.
	Config struct {
		System    SysConfig   `json:"system"`    // Application runtime settings.
		Log       LogConfig   `json:"log"`       // Logger output settings.
		Databases []Databases `json:"databases"` // Database connection settings.
		Cache     Cache       `json:"cache"`     // Cache settings.
		Redis     []Redis     `json:"redis"`     // Redis client settings.
		Monitor   Monitor     `json:"monitor"`   // Panic and alert monitor settings.
		Feishu    Feishu      `json:"feishu"`    // Feishu integration settings.
		Collector Collector   `json:"collector"` // Docker log collector settings.
	}

	// LogConfig controls logger driver and severity level.
	LogConfig struct {
		Driver  string `json:"driver"` // Logger driver, such as "stdout" or "file".
		Level   string `json:"level"`  // Log level: debug, info, warn, error, fatal.
		LogPath string `json:"path"`   // Log file path when driver is "file".
	}

	// SysConfig stores basic runtime properties for the service.
	SysConfig struct {
		Name         string        `json:"name"`          // Service name.
		RunMode      string        `json:"run_mode"`      // Gin run mode.
		HTTPPort     string        `json:"http_port"`     // HTTP listen address.
		ReadTimeout  time.Duration `json:"read_timeout"`  // Maximum request read timeout in seconds.
		WriteTimeout time.Duration `json:"write_timeout"` // Maximum response write timeout in seconds.
		Version      string        `json:"version"`       // Service version.
		RootPath     string        `json:"root_path"`     // Runtime root path.
		DebugMode    bool          `json:"debug_mode"`    // Debug mode toggle.
		LangDir      string        `json:"lang_dir"`      // i18n language files directory.
		DefaultLang  string        `json:"default_lang"`  // Default language key.
		EnvKey       string        `json:"env_key"`       // Environment variable key that stores run env.
		JwtSecret    string        `json:"jwt_secret"`    // Secret key for JWT signing.
		TokenExpire  time.Duration `json:"token_expire"`  // JWT expiration time in seconds.
		Env          string        `json:"env"`           // Resolved runtime environment.
	}

	// Databases stores one database connection profile.
	Databases struct {
		Enable                 bool          `json:"enable"`                              // Whether this DB profile is enabled.
		DbType                 string        `json:"db_type"`                             // Database type, such as mysql.
		DbHost                 string        `json:"db_host"`                             // Database host.
		DbName                 string        `json:"db_name"`                             // Database name.
		DbUsername             string        `json:"db_username,omitempty"`               // Database username.
		DbPassword             string        `json:"db_password,omitempty"`               // Database password.
		DbMaxIdleConn          int           `json:"db_max_idle_conn,omitempty"`          // Maximum idle connections.
		DbMaxOpenConn          int           `json:"db_max_open_conn,omitempty"`          // Maximum open connections.
		DbMaxLifetime          time.Duration `json:"db_max_lifetime,omitempty"`           // Connection max lifetime in hours.
		DbConnectRetryCount    int           `json:"db_connect_retry_count,omitempty"`    // Retry count when DB initialization fails.
		DbConnectRetryInterval int           `json:"db_connect_retry_interval,omitempty"` // Retry interval in seconds.
	}

	// Cache holds global cache settings.
	Cache struct {
		Driver string `json:"driver"` // Cache driver name.
		Prefix string `json:"prefix"` // Cache key prefix.
	}

	// Redis stores one Redis connection profile.
	Redis struct {
		Name        string        `json:"name"`         // Redis connection alias.
		Enable      bool          `json:"enable"`       // Whether this Redis profile is enabled.
		Host        string        `json:"host"`         // Redis host.
		Auth        string        `json:"auth"`         // Redis password or auth token.
		MaxIdle     int           `json:"max_idle"`     // Maximum idle connections.
		MaxActive   int           `json:"max_active"`   // Maximum active connections.
		IdleTimeout time.Duration `json:"idle_timeout"` // Idle timeout in minutes.
		Prefix      string        `json:"prefix"`       // Redis key prefix.
		DB          int           `json:"db"`
	}

	Monitor struct {
		PanicRobot PanicRobot `json:"panic_robot"`
	}

	PanicRobot struct {
		Enable bool        `json:"enable"`
		Wechat robotConfig `json:"wechat"`
		Feishu robotConfig `json:"feishu"`
	}

	robotConfig struct {
		Enable  bool   `json:"enable"`
		PushUrl string `json:"push_url"`
	}

	Feishu struct {
		Enable       bool   `json:"enable"`
		GroupWebhook string `json:"group_webhook"`
		AppID        string `json:"app_id"`
		AppSecret    string `json:"app_secret"`
		EncryptKey   string `json:"encrypt_key"`
	}

	// Collector controls Docker log collection behavior.
	Collector struct {
		MonitorSelf              bool     `json:"monitor_self"`                // Whether to monitor this service container itself.
		UnstructuredLogLineFlags []string `json:"unstructured_log_line_flags"` // Prefix flags recognized as unstructured logs.
		TimeLayout               []string `json:"time_layout"`                 // Supported log time formats.
		ContainerName            []string `json:"container_name"`              // Container names to collect logs from.
	}
)

// LoadConfig loads configuration from bin/configs/<RUN_ENV>.json.
//
// Returns:
//   - *Config: parsed configuration instance also stored globally.
//   - error: returned when reading or decoding configuration fails.
//
// Behavior:
//   - Uses "local" when RUN_ENV is not provided.
//   - Applies APP_NAME override when present.
//
// Example:
//
//	cfg, err := app.LoadConfig()
//	if err != nil {
//		panic(err)
//	}
func LoadConfig() (*Config, error) {
	var (
		runEnv     string
		appName    string
		rootPath   string
		cfgContent []byte
		err        error
	)

	runEnv = os.Getenv(envKey)
	if runEnv == "" {
		runEnv = "local"
	}

	rootPath, err = os.Getwd()
	if err != nil {
		log.Fatalf("无法获取工作目录: %v", err)
	}

	// Build the environment-specific configuration file path.
	configFilePath := filepath.Join(rootPath, "bin", "configs", fmt.Sprintf("%s.json", runEnv))
	cfgContent, err = os.ReadFile(configFilePath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(cfgContent, &config)
	if err != nil {
		return nil, err
	}

	appName = os.Getenv(nameKey)
	if appName != "" {
		config.System.Name = appName
	}

	config.System.Env = runEnv
	config.System.RootPath = rootPath
	config.System.EnvKey = envKey
	config.System.LangDir = filepath.Join(rootPath, "bin", "lang")

	checkConfig(config)

	return config, nil
}

// checkConfig validates required runtime configuration fields.
//
// Parameters:
//   - conf: configuration object to validate.
//
// Returns:
//   - None.
func checkConfig(conf *Config) {
	if conf.System.JwtSecret == "" {
		log.Panicf("JwtSecret Can not be null")
	}
}

// GetConfig returns the globally loaded configuration singleton.
//
// Returns:
//   - *Config: configuration instance loaded by LoadConfig.
func GetConfig() *Config {
	return config
}
