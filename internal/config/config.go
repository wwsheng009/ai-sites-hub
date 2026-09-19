// Package config 加载配置：YAML 文件 + AISC_ 前缀环境变量覆盖。
package config

import (
	"fmt"
	"os"
	"path/filepath"
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
	Proxy    ProxyConfig    `mapstructure:"proxy"`
}

// ProxyConfig 全局出站代理（站点级 proxy_url 为空时回落）。
type ProxyConfig struct {
	// URL 支持 http/https/socks5；空=直连。环境变量 AISC_PROXY_URL 可覆盖。
	URL string `mapstructure:"url"`
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
	// AutoMigrate 启动时自动应用 migrations/*.sql 的未应用迁移（默认 true）。
	// 设为 false 时需手动运行 `aiclient migrate`。
	AutoMigrate bool `mapstructure:"auto_migrate"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type JobsConfig struct {
	Enabled       bool `mapstructure:"enabled"`
	DryRunDefault bool `mapstructure:"dry_run_default"`
}

// Load 读取配置。path 非空时读单个文件；为空时按优先级从低到高合并查找：
// config.example.yaml → config.yaml → config.local.yaml（根目录与 configs/ 都找，根目录优先），
// 高优先级文件的同名键覆盖低优先级；全部不存在时用默认值。
// 所有键均可被 AISC_ 前缀环境变量覆盖（如 AISC_SECURITY_MASTER_KEY）。
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
		// 候选文件按优先级从低到高依次合并（后合并者覆盖先合并者的同名键）。
		// 合并走 viper 的 config 层，AISC_ 环境变量的覆盖优先级仍高于文件。
		for _, p := range searchCandidates() {
			v.SetConfigFile(p)
			if err := v.MergeInConfig(); err != nil {
				return nil, fmt.Errorf("合并配置 %s: %w", p, err)
			}
		}
		// 一个候选都不存在：用默认值继续（doctor 会提示）
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置: %w", err)
	}
	return &cfg, nil
}

// searchCandidates 返回存在的候选配置文件，按优先级从低到高排列：
// 同名文件根目录优先于 configs/；文件类型优先级 example < config < config.local。
func searchCandidates() []string {
	var found []string
	for _, name := range []string{"config.example.yaml", "config.yaml", "config.local.yaml"} {
		for _, dir := range []string{"configs", "."} {
			p := filepath.Join(dir, name)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				found = append(found, p)
			}
		}
	}
	return found
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.addr", ":8080")
	v.SetDefault("security.master_key", "")
	v.SetDefault("database.path", "data/aiclient.db")
	v.SetDefault("database.auto_migrate", true)
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "console")
	v.SetDefault("jobs.enabled", false)
	v.SetDefault("jobs.dry_run_default", true)
	v.SetDefault("proxy.url", "")
}
