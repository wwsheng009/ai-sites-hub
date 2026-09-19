// Package logger 构建全局 zap 日志（NFR：脱敏由调用方保证，zap 不打凭据字段）。
package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New 按配置构建 zap.Logger。
func New(level, format string) (*zap.Logger, error) {
	lvl, err := zapcore.ParseLevel(level)
	if err != nil {
		lvl = zapcore.InfoLevel
	}

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var core zapcore.Core
	if format == "json" {
		core = zapcore.NewCore(zapcore.NewJSONEncoder(encCfg), zapcore.Lock(os.Stdout), lvl)
	} else {
		encCfg.EncodeLevel = zapcore.CapitalLevelEncoder
		core = zapcore.NewCore(zapcore.NewConsoleEncoder(encCfg), zapcore.Lock(os.Stdout), lvl)
	}

	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)), nil
}

// Nop 返回无输出 logger（测试用）。
func Nop() *zap.Logger {
	return zap.NewNop()
}
