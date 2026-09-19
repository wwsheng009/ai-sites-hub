// Package config 加载配置：YAML 文件 + AISC_ 前缀环境变量覆盖。
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config 顶层配置（对齐 configs/config.example.yaml）。
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Security SecurityConfig `mapstructure:"security"`
	Database DatabaseConfig `mapstructure:"database"`
	Log      LogConfig      `mapstructure:"log"`
	Jobs     JobsConfig     `mapstructure:"jobs"`
}

type ServerConfig struct {
	Addr     string `mapstructure:"addr"`
	BasePath string `mapstructure:"base_path"`
}

type SecurityConfig struct {
	// MasterKey 为空时：优先读环境变量 AISC_SECURITY_MASTER_KEY；
	// doctor 会报告"未配置主密钥"（凭据写入 API 将拒绝服务）。
	MasterKey string `mapstructure:"master_key"`
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type JobsConfig struct {
	Enabled       bool `mapstructure:"enabled"`
	DryRunDefault bool `mapstructure:"dry_run_default"`
}

// Load 读取配置。path 为空时按 config.yaml → config.example.yaml 顺序查找（均不存在则用默认值）。
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix("AISC")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("读取配置 %s: %w", path, err)
		}
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("configs")
		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, fmt.Errorf("读取配置: %w", err)
			}
			// 无 config.yaml：用默认值继续（doctor 会提示）
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置: %w", err)
	}
	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.addr", ":8080")
	v.SetDefault("security.master_key", "")
	v.SetDefault("database.path", "data/aiclient.db")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "console")
	v.SetDefault("jobs.enabled", false)
	v.SetDefault("jobs.dry_run_default", true)
}
