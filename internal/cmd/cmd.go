// Package cmd cobra 命令组装（root + serve + doctor + migrate + version）。
package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"aiclient/internal/config"
	"aiclient/internal/httpx"
	"aiclient/internal/logger"
	"aiclient/internal/svcwire"
	"aiclient/internal/webui"
)

var cfgFile string

// version/buildInfo 发布构建时由 -ldflags 注入（scripts/build.ps1 -Version）。
var (
	version   = "0.1.0"
	buildInfo = "dev"
)

var rootCmd = &cobra.Command{
	Use:           "aiclient",
	Short:         "AI 中转站点集中管理平台（sub2api / new-api 多站点管理）",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute 运行根命令。
func Execute() error { return rootCmd.Execute() }

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路径（默认 ./configs/config.yaml，可用 AISC_CONFIG 覆盖）")
	rootCmd.AddCommand(serveCmd, doctorCmd, migrateCmd, versionCmd)
}

// loadConfig 加载配置（file + AISC_ 环境变量）。
func loadConfig() (*config.Config, error) {
	if cfgFile == "" {
		cfgFile = os.Getenv("AISC_CONFIG")
	}
	return config.Load(cfgFile)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "打印版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		front := "api-only"
		if webui.HasFrontend() {
			front = "webui-embedded"
		}
		fmt.Printf("aiclient %s (%s, %s)\n", version, buildInfo, front)
	},
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "执行数据库迁移（幂等）",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		w, err := svcwire.New(cfg)
		if err != nil {
			return err
		}
		defer w.Close()

		applied, err := w.Migrate(cmd.Context())
		if err != nil {
			return err
		}
		fmt.Printf("迁移完成：本次应用 %d 个迁移文件\n", len(applied))
		for _, f := range applied {
			fmt.Println("  -", f)
		}
		return nil
	},
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "环境体检（主密钥/数据库/登录态）",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		w, err := svcwire.New(cfg)
		if err != nil {
			return err
		}
		defer w.Close()

		rep := w.Services.Doctor(cmd.Context())
		for _, ck := range rep.Checks {
			mark := "[ok]"
			if ck.Status == "warn" {
				mark = "[warn]"
			} else if ck.Status == "fail" {
				mark = "[fail]"
			}
			fmt.Printf("%-9s %-12s %s\n", mark, ck.Name, ck.Detail)
		}
		if rep.Overall != "ok" {
			return fmt.Errorf("doctor 未通过（overall=%s）", rep.Overall)
		}
		fmt.Println("doctor 全部通过")
		return nil
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "启动 HTTP API 服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		w, err := svcwire.New(cfg)
		if err != nil {
			return err
		}
		defer w.Close()

		zlog, err := logger.New(cfg.Log.Level, cfg.Log.Format)
		if err != nil {
			return err
		}

		router := httpx.SetupRouter(w.Services)
		httpx.AttachSPA(router, webui.Dist())
		srv := &http.Server{Addr: cfg.Server.Addr, Handler: router}

		errCh := make(chan error, 1)
		go func() {
			zlog.Sugar().Infof("HTTP 服务启动 %s", cfg.Server.Addr)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}()

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		select {
		case err := <-errCh:
			return fmt.Errorf("HTTP 服务异常退出: %w", err)
		case <-quit:
		}

		zlog.Sugar().Info("收到退出信号，优雅关闭…")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	},
}
