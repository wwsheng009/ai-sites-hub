// Package store SQLite 连接与迁移执行（glebarez 纯 Go driver，零 CGO）。
package store

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/glebarez/go-sqlite" // database/sql driver
	gormsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// DB 同时暴露 GORM 与 database/sql 句柄。
type DB struct {
	GORM *gorm.DB
	SQL  *sql.DB
}

// Open 打开（并确保父目录存在）SQLite 数据库。
func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("store: 创建数据目录: %w", err)
		}
	}
	gdb, err := gorm.Open(gormsqlite.Open(path), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("store: 打开 sqlite %s: %w", path, err)
	}
	sdb, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("store: 获取 sql.DB: %w", err)
	}
	// 单文件库：限制连接数，避免锁竞争
	sdb.SetMaxOpenConns(1)
	return &DB{GORM: gdb, SQL: sdb}, nil
}

// Migrate 按 migrations/*.sql 文件名字典序执行未应用的迁移。
// 幂等：已应用版本记录在 schema_migrations；SQL 本身全部使用 IF NOT EXISTS。
func (d *DB) Migrate(ctx context.Context, dir string) (applied []string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("store: 读取迁移目录 %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)

	for _, f := range files {
		appliedFlag, err := d.checkApplied(ctx, f)
		if err != nil {
			return applied, err
		}
		if appliedFlag {
			continue
		}
		if err := d.applyOne(ctx, filepath.Join(dir, f), f); err != nil {
			return applied, fmt.Errorf("store: 迁移 %s 失败: %w", f, err)
		}
		applied = append(applied, f)
	}
	return applied, nil
}

func (d *DB) checkApplied(ctx context.Context, version string) (bool, error) {
	var n int
	row := d.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'`)
	if err := row.Scan(&n); err != nil {
		return false, fmt.Errorf("store: 检查 schema_migrations: %w", err)
	}
	if n == 0 {
		return false, nil
	}
	row = d.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version)
	if err := row.Scan(&n); err != nil {
		return false, fmt.Errorf("store: 查询迁移版本: %w", err)
	}
	return n > 0, nil
}

func (d *DB) applyOne(ctx context.Context, path, version string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取迁移文件: %w", err)
	}
	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启事务: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 逐语句执行（database/sql 驱动不保证多语句）；语句间用空行/分号边界拆分
	for _, stmt := range splitStatements(string(content)) {
		if stmt == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("执行语句 [%s…]: %w", truncate(stmt, 48), err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations(version) VALUES (?)`, version); err != nil {
		return fmt.Errorf("记录迁移版本: %w", err)
	}
	return tx.Commit()
}

// splitStatements 粗粒度拆分 SQL 语句（本项目迁移语句均为单行/多行 CREATE，不含触发器 BEGIN..END）。
func splitStatements(content string) []string {
	sc := bufio.NewScanner(strings.NewReader(content))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	var stmts []string
	var cur strings.Builder
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") { // 注释行
			continue
		}
		cur.WriteString(line)
		cur.WriteString("\n")
		if strings.HasSuffix(trimmed, ";") {
			stmts = append(stmts, strings.TrimSpace(cur.String()))
			cur.Reset()
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		stmts = append(stmts, s)
	}
	return stmts
}

func truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Close 关闭底层连接。
func (d *DB) Close() error {
	return d.SQL.Close()
}
