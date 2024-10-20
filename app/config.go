// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

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

var config *Config

type (
	Config struct {
		System    SysConfig   `json:"system"`    // 应用系统配置
		Log       LogConfig   `json:"log"`       // 日志配置
		Databases []Databases `json:"databases"` // 数据库配置
		Cache     Cache       `json:"cache"`     // 缓存配置
		Redis     []Redis     `json:"redis"`     // Redis 配置
		Monitor   Monitor     `json:"monitor"`   // 监控配置
		Feishu    Feishu      `json:"feishu"`    // 飞书配置
		Collector Collector   `json:"collector"` // 日志采集器配置
	}

	LogConfig struct {
		Driver  string `json:"driver"`   // 日志驱动 stdout, file
		Level   string `json:"level"`    // 日志级别 debug,info,warn,error,fatal
		LogPath string `json:"log_path"` // 日志路径，仅当Driver为file时生效
	}

	SysConfig struct {
		Name         string        `json:"name"`          // 应用名称
		RunMode      string        `json:"run_mode"`      // 运行模式
		HTTPPort     string        `json:"http_port"`     // 端口号
		ReadTimeout  time.Duration `json:"read_timeout"`  // 请求最大超时时间
		WriteTimeout time.Duration `json:"write_timeout"` // 响应最大超时时间
		Version      string        `json:"version"`       // 版本号
		RootPath     string        `json:"root_path"`     // 根目录
		DebugMode    bool          `json:"debug_mode"`    // 调试模式
		LangDir      string        `json:"lang_dir"`      // 语言目录
		DefaultLang  string        `json:"default_lang"`  // 默认语言
		EnvKey       string        `json:"env_key"`       // 运行环境key，用来读取运行环境
		JwtSecret    string        `json:"jwt_secret"`    // 鉴权服务JwtSecret
		TokenExpire  time.Duration `json:"token_expire"`  // 鉴权服务token过期时间(秒)
		Env          string        `json:"env"`           // 运行环境
	}

	Databases struct {
		Enable        bool          `json:"enable"`                     // 开关
		DbType        string        `json:"db_type"`                    // 类型
		DbHost        string        `json:"db_host"`                    // HOST
		DbName        string        `json:"db_name"`                    // 数据库名称
		DbUsername    string        `json:"db_username,omitempty"`      // 数据库用户名
		DbPassword    string        `json:"db_password,omitempty"`      // 数据库用户密码
		DbMaxIdleConn int           `json:"db_max_idle_conn,omitempty"` // 空闲连接池中连接的最大数量
		DbMaxOpenConn int           `json:"db_max_open_conn,omitempty"` // 数据库连接的最大数量
		DbMaxLifetime time.Duration `json:"db_max_lifetime,omitempty"`  // 连接可复用的最大时间（单位：小时）
	}

	Cache struct {
		Driver string `json:"driver"` // 缓存驱动
		Prefix string `json:"prefix"` // 缓存前缀
	}

	Redis struct {
		Name        string        `json:"name"`         // Redis连接名
		Enable      bool          `json:"enable"`       // 开关
		Host        string        `json:"host"`         // HOST
		Auth        string        `json:"auth"`         // 授权
		MaxIdle     int           `json:"max_idle"`     // 最大空闲连接数
		MaxActive   int           `json:"max_active"`   // 一个pool所能分配的最大的连接数目
		IdleTimeout time.Duration `json:"idle_timeout"` // 空闲连接超时时间，超过超时时间的空闲连接会被关闭（单位：分钟）
		Prefix      string        `json:"prefix"`       // 前缀
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

	Collector struct {
		MonitorSelf              bool     `json:"monitor_self"`                // 是否监视本应用
		UnstructuredLogLineFlags []string `json:"unstructured_log_line_flags"` // 支持解析的非结构日志行标记
		TimeLayout               []string `json:"time_layout"`                 // 支持解析的日志时间格式
		ContainerName            []string `json:"container_name"`              // 需要采集的容器名称
	}
)

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

	// 拼接配置文件路径
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

func checkConfig(conf *Config) {
	if conf.System.JwtSecret == "" {
		log.Panicf("JwtSecret Can not be null")
	}
}

func GetConfig() *Config {
	return config
}
