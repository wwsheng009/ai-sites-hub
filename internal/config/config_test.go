// Package config 加载逻辑单测：搜索链优先级、环境变量覆盖、默认值。
package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile 在 path（自动建父目录）写入 YAML 文本。
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

// neutralizeEnv 置空测试涉及的 AISC_ 环境变量（viper 默认忽略空值，等同未设置），
// 避免开发机全局环境变量影响断言。
func neutralizeEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"AISC_SERVER_ADDR", "AISC_SECURITY_MASTER_KEY", "AISC_DATABASE_PATH", "AISC_LOG_LEVEL",
	} {
		t.Setenv(k, "")
	}
}

func mustLoad(t *testing.T) *Config {
	t.Helper()
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

// 优先级链：config.example.yaml < config.yaml < config.local.yaml（高覆盖低）。
func TestLoadSearchChainLocalOverridesBase(t *testing.T) {
	neutralizeEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)

	writeFile(t, filepath.Join(dir, "configs", "config.example.yaml"),
		"server:\n  addr: \":1000\"\nsecurity:\n  master_key: example-key\n")
	writeFile(t, filepath.Join(dir, "config.yaml"),
		"server:\n  addr: \":2000\"\nlog:\n  level: debug\n")
	writeFile(t, filepath.Join(dir, "config.local.yaml"),
		"security:\n  master_key: local-key\n")

	cfg := mustLoad(t)
	if cfg.Server.Addr != ":2000" {
		t.Errorf("server.addr 应取 config.yaml 的 :2000，got %q", cfg.Server.Addr)
	}
	if cfg.Security.MasterKey != "local-key" {
		t.Errorf("security.master_key 应取 config.local.yaml，got %q", cfg.Security.MasterKey)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("log.level 应取 config.yaml 的 debug，got %q", cfg.Log.Level)
	}
}

// 同名文件根目录优先于 configs/；configs/config.local.yaml 同样参与合并。
func TestLoadRootBeatsConfigsDir(t *testing.T) {
	neutralizeEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)

	writeFile(t, filepath.Join(dir, "configs", "config.yaml"), "server:\n  addr: \":3000\"\n")
	writeFile(t, filepath.Join(dir, "config.yaml"), "server:\n  addr: \":3100\"\n")
	writeFile(t, filepath.Join(dir, "configs", "config.local.yaml"), "log:\n  level: warn\n")

	cfg := mustLoad(t)
	if cfg.Server.Addr != ":3100" {
		t.Errorf("server.addr 根目录应优先于 configs/，got %q", cfg.Server.Addr)
	}
	if cfg.Log.Level != "warn" {
		t.Errorf("log.level 应取 configs/config.local.yaml，got %q", cfg.Log.Level)
	}
}

// 只有 config.local.yaml 时也能被发现；缺失键回落默认值。
func TestLoadLocalOnly(t *testing.T) {
	neutralizeEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)

	writeFile(t, filepath.Join(dir, "config.local.yaml"), "security:\n  master_key: only-local\n")

	cfg := mustLoad(t)
	if cfg.Security.MasterKey != "only-local" {
		t.Errorf("master_key 应取 config.local.yaml，got %q", cfg.Security.MasterKey)
	}
	if cfg.Server.Addr != ":8080" {
		t.Errorf("缺失键应回落默认 :8080，got %q", cfg.Server.Addr)
	}
}

// 环境变量优先于任何配置文件。
func TestLoadEnvOverridesFiles(t *testing.T) {
	neutralizeEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)

	writeFile(t, filepath.Join(dir, "config.local.yaml"), "security:\n  master_key: local-key\n")
	t.Setenv("AISC_SECURITY_MASTER_KEY", "env-key")

	if cfg := mustLoad(t); cfg.Security.MasterKey != "env-key" {
		t.Errorf("环境变量应覆盖文件值，got %q", cfg.Security.MasterKey)
	}
}

// 无任何配置文件：默认值、不报错。
func TestLoadNoFilesDefaults(t *testing.T) {
	neutralizeEnv(t)
	t.Chdir(t.TempDir())

	cfg := mustLoad(t)
	if cfg.Security.MasterKey != "" {
		t.Errorf("无文件时 master_key 应为空，got %q", cfg.Security.MasterKey)
	}
	if cfg.Server.Addr != ":8080" {
		t.Errorf("无文件时 server.addr 应为默认 :8080，got %q", cfg.Server.Addr)
	}
	if cfg.Database.Path != "data/aiclient.db" {
		t.Errorf("无文件时 database.path 应为默认，got %q", cfg.Database.Path)
	}
	if !cfg.Database.AutoMigrate {
		t.Errorf("无文件时 database.auto_migrate 应默认为 true")
	}
}

// 显式 --config 单文件路径不受搜索链影响。
func TestLoadExplicitPath(t *testing.T) {
	neutralizeEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)

	writeFile(t, filepath.Join(dir, "config.local.yaml"), "server:\n  addr: \":8888\"\n")
	p := filepath.Join(dir, "custom.yaml")
	writeFile(t, p, "server:\n  addr: \":9000\"\n")

	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load(%s): %v", p, err)
	}
	if cfg.Server.Addr != ":9000" {
		t.Errorf("显式路径应完全生效，got %q", cfg.Server.Addr)
	}
}
