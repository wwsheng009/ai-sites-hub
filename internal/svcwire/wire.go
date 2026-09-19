// Package svcwire 组装层：config → logger → store → secret → repo → registry → service。
// 独立于具体入口（cobra/gin），供 serve/doctor/migrate 与未来 jobs 调度共用。
package svcwire

import (
	"context"
	"log/slog"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"aiclient/internal/adapter"
	"aiclient/internal/adapter/newapi"
	"aiclient/internal/adapter/sub2api"
	"aiclient/internal/config"
	"aiclient/internal/logger"
	"aiclient/internal/repo"
	"aiclient/internal/secret"
	"aiclient/internal/service"
	"aiclient/internal/service/sitedetect"
	"aiclient/internal/store"
)

// Wire 组装结果（store/cipher 句柄供未来 jobs 复用）。
type Wire struct {
	Services *service.Services
	Log      *slog.Logger

	db     *store.DB
	cipher *secret.Cipher
}

// Close 释放资源。
func (w *Wire) Close() { _ = w.db.Close() }

// New 构建组装结果（store/cipher 句柄供未来 jobs 复用）。
func New(cfg *config.Config) (*Wire, error) {
	zapLog, err := logger.New(cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		return nil, err
	}
	// service/adapter 统一走 slog 门面（zap 作为底层 sink）
	log := slog.New(zapSlogHandler{zapLog})

	st, err := store.Open(cfg.Database.Path)
	if err != nil {
		return nil, err
	}

	var cipher *secret.Cipher
	if cfg.Security.MasterKey != "" {
		cipher, err = secret.NewCipher([]byte(cfg.Security.MasterKey))
		if err != nil {
			_ = st.Close()
			return nil, err
		}
	}

	rep := repo.New(st.GORM)

	reg := adapter.NewRegistry()
	reg.Register(adapter.TypeSub2API, func(base, proxyURL string, l *slog.Logger) (adapter.SiteAdapter, error) {
		return sub2api.New(base, proxyURL, l)
	})
	reg.Register(adapter.TypeNewAPI, func(base, proxyURL string, l *slog.Logger) (adapter.SiteAdapter, error) {
		return newapi.New(base, proxyURL, l)
	})

	svcs := &service.Services{
		Repo:        rep,
		Sec:         cipher,
		Reg:         reg,
		Detect:      sitedetect.New(reg, log),
		Log:         log,
		GlobalProxy: strings.TrimSpace(cfg.Proxy.URL),
	}
	return &Wire{Services: svcs, Log: log, db: st, cipher: cipher}, nil
}

// MigrateDir 迁移目录（供 cmd migrate 使用）。
func (w *Wire) MigrateDir() string { return "migrations" }

// zapSlogHandler 把 slog 记录桥接到 zap。
type zapSlogHandler struct{ z *zap.Logger }

func (h zapSlogHandler) Enabled(_ context.Context, l slog.Level) bool {
	return h.z.Core().Enabled(zapLevel(l))
}

func (h zapSlogHandler) Handle(_ context.Context, r slog.Record) error {
	ze := zapFields(r)
	h.z.Log(zapLevel(r.Level), r.Message, ze...)
	return nil
}

func (h zapSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h // M2 骨架：不支持属性挂载（调用方直接传结构化字段即可）
}

func (h zapSlogHandler) WithGroup(name string) slog.Handler { return h }

func zapLevel(l slog.Level) zapcore.Level {
	switch {
	case l >= slog.LevelError:
		return zapcore.ErrorLevel
	case l >= slog.LevelWarn:
		return zapcore.WarnLevel
	case l >= slog.LevelDebug:
		return zapcore.DebugLevel
	default:
		return zapcore.InfoLevel
	}
}

func zapFields(r slog.Record) []zap.Field {
	fields := make([]zap.Field, 0, r.NumAttrs()+1)
	r.Attrs(func(a slog.Attr) bool {
		fields = append(fields, zap.Any(a.Key, a.Value.Any()))
		return true
	})
	return fields
}

// Migrate 执行迁移（供 cmd migrate 调用；返回本次应用的迁移文件）。
func (w *Wire) Migrate(ctx context.Context) ([]string, error) {
	return w.db.Migrate(ctx, "migrations")
}
