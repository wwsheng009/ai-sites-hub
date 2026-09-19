// aiclient 命令行入口（architecture §7：serve / doctor / migrate / version / stop）。
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
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

		// 同步调度器
		schedCtx, schedCancel := context.WithCancel(context.Background())
		w.Scheduler.Start(schedCtx)
		defer func() {
			schedCancel()
			w.Scheduler.Stop()
		}()

		addr := cfg.Server.Addr
		if host, _ := cmd.Flags().GetString("host"); host != "" {
			if port, _ := cmd.Flags().GetInt("port"); port != 0 {
				addr = net.JoinHostPort(host, strconv.Itoa(port))
			} else {
				addr = net.JoinHostPort(host, strconv.Itoa(extractPort(addr)))
			}
		} else if port, _ := cmd.Flags().GetInt("port"); port != 0 {
			addr = net.JoinHostPort(extractHost(addr), strconv.Itoa(port))
		}

		srv := &http.Server{Addr: addr, Handler: router}
		httpx.RegisterQuitHandler(router, func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		})

		if err := writePIDFile(addr); err != nil {
			zlog.Sugar().Warnw("pid 文件写入失败", "err", err)
		}
		defer removePIDFile()

		done := make(chan struct{})
		var once sync.Once
		errCh := make(chan error, 1)
		go func() {
			zlog.Sugar().Infof("HTTP 服务启动 %s", addr)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				select {
				case errCh <- err:
				default:
				}
			}
			once.Do(func() { close(done) })
		}()

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		select {
		case err := <-errCh:
			return fmt.Errorf("HTTP 服务异常退出: %w", err)
		case <-quit:
			zlog.Sugar().Info("收到退出信号，优雅关闭…")
		case <-done:
			zlog.Sugar().Info("HTTP 服务已停止")
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "停止正在运行的 aiclient 服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		url, _ := cmd.Flags().GetString("url")
		if url == "" {
			addr, _, err := readPIDFile()
			if err != nil {
				return fmt.Errorf("无法读取 pid 文件 %s（请用 --url 指定地址）: %w", pidFile, err)
			}
			url = resolvePIDURL(addr)
		}
		quitURL := url + "/-/quit"
		fmt.Printf("正在停止服务（%s）…\n", url)
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, quitURL, nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("无法连接到服务（%s）: %w", quitURL, err)
		}
		resp.Body.Close()
		pollURL := url + "/api/v1/health"
		deadline := time.After(10 * time.Second)
		tick := time.NewTicker(200 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-deadline:
				return fmt.Errorf("服务未在 10s 内停止（%s）", url)
			case <-tick.C:
				if _, err := http.Get(pollURL); err != nil {
					removePIDFile()
					fmt.Println("服务已停止")
					return nil
				}
			}
		}
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
		// migrate 命令以显式调用为准，关闭启动时自动迁移，避免重复执行。
		cfg.Database.AutoMigrate = false
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

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路径（为空时按 config.local.yaml → config.yaml → config.example.yaml 优先级合并；也可用 AISC_CONFIG 指定）")
	serveCmd.Flags().String("host", "", "监听主机（覆盖配置 server.addr 中的 host 部分）")
	serveCmd.Flags().Int("port", 0, "监听端口（覆盖配置 server.addr 中的 port 部分）")
	stopCmd.Flags().String("url", "", "服务地址（默认从 pid 文件读取）")
	rootCmd.AddCommand(serveCmd, doctorCmd, migrateCmd, versionCmd, stopCmd)
}

// loadConfig 加载配置（file + AISC_ 环境变量）。
func loadConfig() (*config.Config, error) {
	if cfgFile == "" {
		cfgFile = os.Getenv("AISC_CONFIG")
	}
	return config.Load(cfgFile)
}

const pidFile = "data/aiclient.pid"

type pidRecord struct {
	Addr string `json:"addr"`
	PID  int    `json:"pid"`
}

func writePIDFile(addr string) error {
	if err := os.MkdirAll(filepath.Dir(pidFile), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(pidRecord{Addr: addr, PID: os.Getpid()})
	if err != nil {
		return err
	}
	return os.WriteFile(pidFile, data, 0o644)
}

func removePIDFile() {
	_ = os.Remove(pidFile)
}

func readPIDFile() (string, int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return "", 0, err
	}
	var r pidRecord
	if err := json.Unmarshal(data, &r); err != nil {
		return "", 0, err
	}
	return r.Addr, r.PID, nil
}

func resolvePIDURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr
	}
	if host == "" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func extractHost(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "0.0.0.0"
	}
	if host == "" {
		return "0.0.0.0"
	}
	return host
}

func extractPort(addr string) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 8080
	}
	n, _ := strconv.Atoi(port)
	if n == 0 {
		return 8080
	}
	return n
}
