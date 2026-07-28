// Package repository 提供应用程序的基础设施层组件。
// 包括数据库连接初始化、ORM 客户端管理、Redis 连接、数据库迁移等核心功能。
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/migrations"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/lib/pq"

	_ "modernc.org/sqlite"
)

// InitEnt 初始化 Ent ORM 客户端并返回客户端实例和底层的 *sql.DB。
//
// 该函数执行以下操作：
//  1. 初始化全局时区设置，确保时间处理一致性
//  2. 建立 PostgreSQL 或 SQLite 数据库连接
//  3. 自动执行数据库迁移 / DIY schema bootstrap，确保 schema 与代码同步
//  4. 创建并返回 Ent 客户端实例
//
// 重要提示：调用者必须负责关闭返回的 ent.Client（关闭时会自动关闭底层的 driver/db）。
//
// 参数：
//   - cfg: 应用程序配置，包含数据库连接信息和时区设置
//
// 返回：
//   - *ent.Client: Ent ORM 客户端，用于执行数据库操作
//   - *sql.DB: 底层的 SQL 数据库连接，可用于直接执行原生 SQL
//   - error: 初始化过程中的错误
func InitEnt(cfg *config.Config) (*ent.Client, *sql.DB, error) {
	// 优先初始化时区设置，确保所有时间操作使用统一的时区。
	// 这对于跨时区部署和日志时间戳的一致性至关重要。
	if err := timezone.Init(cfg.Timezone); err != nil {
		return nil, nil, err
	}

	if cfg.IsDIY() || cfg.Database.IsSQLite() {
		return initEntSQLite(cfg)
	}
	return initEntPostgres(cfg)
}

func initEntPostgres(cfg *config.Config) (*ent.Client, *sql.DB, error) {
	SetActiveDialect(DialectPostgres)
	// 构建包含时区信息的数据库连接字符串 (DSN)。
	// 时区信息会传递给 PostgreSQL，确保数据库层面的时间处理正确。
	dsn := cfg.Database.DSNWithTimezone(cfg.Timezone)

	// 使用 Ent 的 SQL 驱动打开 PostgreSQL 连接。
	// dialect.Postgres 指定使用 PostgreSQL 方言进行 SQL 生成。
	var drv *entsql.Driver
	if cfg.Server.EnableServerTiming {
		connector, err := pq.NewConnector(dsn)
		if err != nil {
			return nil, nil, err
		}
		drv = entsql.OpenDB(dialect.Postgres, sql.OpenDB(newServerTimingConnector(connector)))
	} else {
		var err error
		drv, err = entsql.Open(dialect.Postgres, dsn)
		if err != nil {
			return nil, nil, err
		}
	}
	applyDBPoolSettings(drv.DB(), cfg)

	// 确保数据库 schema 已准备就绪。
	// SQL 迁移文件是 schema 的权威来源（source of truth）。
	// 这种方式比 Ent 的自动迁移更可控，支持复杂的迁移场景。
	migrationCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := applyMigrationsFS(migrationCtx, drv.DB(), migrations.FS); err != nil {
		_ = drv.Close() // 迁移失败时关闭驱动，避免资源泄露
		return nil, nil, err
	}

	return finalizeEntClient(migrationCtx, drv, cfg)
}

func initEntSQLite(cfg *config.Config) (*ent.Client, *sql.DB, error) {
	SetActiveDialect(DialectSQLite)
	path := cfg.Database.SQLitePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, nil, fmt.Errorf("create sqlite data dir: %w", err)
	}

	// Register PostgreSQL-compatible helpers (NOW, etc.) once per process for modernc.
	registerSQLiteCompatFunctions()

	// modernc driver name is "sqlite" (not "sqlite3").
	db, err := sql.Open("sqlite", cfg.Database.SQLiteDSN())
	if err != nil {
		return nil, nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Apply WAL and other pragmas on every connection-bearing open.
	if err := configureSQLiteWAL(db); err != nil {
		_ = db.Close()
		return nil, nil, err
	}

	// SQLite writers serialize; keep pool tiny for correctness.
	applyDBPoolSettings(db, cfg)

	drv := entsql.OpenDB(dialect.SQLite, db)

	migrationCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// DIY schema source of truth: Ent models (not the PostgreSQL migration history).
	if err := bootstrapSQLiteSchema(migrationCtx, drv); err != nil {
		_ = drv.Close()
		return nil, nil, fmt.Errorf("sqlite schema bootstrap: %w", err)
	}

	log.Printf("sqlite database ready at %s (WAL mode)", path)
	return finalizeEntClient(migrationCtx, drv, cfg)
}

// OpenSQLiteForSetup opens a SQLite database with WAL, runs Ent schema create, and
// returns a client for one-shot install operations (admin bootstrap). Caller must run cleanup.
func OpenSQLiteForSetup(path string) (*ent.Client, func(), error) {
	if strings.TrimSpace(path) == "" {
		path = config.DefaultSQLitePath
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return nil, nil, fmt.Errorf("create sqlite data dir: %w", err)
	}
	SetActiveDialect(DialectSQLite)
	registerSQLiteCompatFunctions()
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := configureSQLiteWAL(db); err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	drv := entsql.OpenDB(dialect.SQLite, db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := bootstrapSQLiteSchema(ctx, drv); err != nil {
		_ = drv.Close()
		return nil, nil, err
	}
	client := ent.NewClient(ent.Driver(drv))
	cleanup := func() {
		_ = client.Close()
	}
	return client, cleanup, nil
}

func configureSQLiteWAL(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA temp_store=MEMORY;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("exec %q: %w", p, err)
		}
	}
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode;").Scan(&mode); err != nil {
		return fmt.Errorf("read journal_mode: %w", err)
	}
	if mode != "wal" {
		return fmt.Errorf("expected journal_mode=wal, got %q", mode)
	}
	return nil
}

// bootstrapSQLiteSchema creates tables from Ent schema definitions and records a
// synthetic migration marker so operators can see the DB was bootstrapped for DIY.
func bootstrapSQLiteSchema(ctx context.Context, drv *entsql.Driver) error {
	client := ent.NewClient(ent.Driver(drv))
	// Do not close client here — it owns the shared driver used by the returned client.

	if err := client.Schema.Create(ctx); err != nil {
		return fmt.Errorf("ent schema create: %w", err)
	}

	// Track DIY bootstrap without replaying PostgreSQL migration files.
	const ddl = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	filename   TEXT PRIMARY KEY,
	checksum   TEXT NOT NULL,
	applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);`
	if _, err := drv.DB().ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}
	// Tables that live only in Postgres migrations (not Ent models) but are hit
	// on auth hot paths. Missing them makes every /auth/me fail with USER_NOT_FOUND.
	if err := ensureSQLiteAuxTables(ctx, drv.DB()); err != nil {
		return err
	}

	const marker = "diy_ent_bootstrap"
	const checksum = "diy-ent-schema-v2"
	_, err := drv.DB().ExecContext(ctx, `
INSERT OR IGNORE INTO schema_migrations (filename, checksum, applied_at)
VALUES (?, ?, datetime('now'))
`, marker, checksum)
	if err != nil {
		return fmt.Errorf("record diy bootstrap: %w", err)
	}
	return nil
}

// ensureSQLiteAuxTables creates migration-only tables required by auth/profile
// raw SQL that is not covered by Ent Schema.Create.
func ensureSQLiteAuxTables(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return nil
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS user_avatars (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL UNIQUE,
			storage_provider TEXT NOT NULL DEFAULT 'database',
			storage_key TEXT NOT NULL DEFAULT '',
			url TEXT NOT NULL DEFAULT '',
			content_type TEXT NOT NULL DEFAULT '',
			byte_size INTEGER NOT NULL DEFAULT 0,
			sha256 TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS user_avatars_user_id_key ON user_avatars (user_id)`,
		`CREATE TABLE IF NOT EXISTS user_provider_default_grants (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			provider_type TEXT NOT NULL,
			grant_reason TEXT NOT NULL DEFAULT 'first_bind',
			granted_at TEXT NOT NULL DEFAULT (datetime('now')),
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE (user_id, provider_type, grant_reason),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS user_provider_default_grants_user_id_idx
			ON user_provider_default_grants (user_id)`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensure sqlite aux table: %w", err)
		}
	}
	return nil
}

func finalizeEntClient(ctx context.Context, drv *entsql.Driver, cfg *config.Config) (*ent.Client, *sql.DB, error) {
	// 创建 Ent 客户端，绑定到已配置的数据库驱动。
	client := ent.NewClient(ent.Driver(drv))

	// 启动阶段：从配置或数据库中确保系统密钥可用。
	if err := ensureBootstrapSecrets(ctx, client, cfg); err != nil {
		_ = client.Close()
		return nil, nil, err
	}

	// 在密钥补齐后执行完整配置校验，避免空 jwt.secret 导致服务运行时失败。
	if err := cfg.Validate(); err != nil {
		_ = client.Close()
		return nil, nil, fmt.Errorf("validate config after secret bootstrap: %w", err)
	}

	// SIMPLE 模式：启动时补齐各平台默认分组。
	// - anthropic/openai/gemini: 确保存在 <platform>-default
	// - antigravity: 仅要求存在 >=2 个未软删除分组（用于 claude/gemini 混合调度场景）
	if cfg.RunMode == config.RunModeSimple {
		seedCtx, seedCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer seedCancel()
		if err := ensureSimpleModeDefaultGroups(seedCtx, client); err != nil {
			_ = client.Close()
			return nil, nil, err
		}
		if err := ensureSimpleModeAdminConcurrency(seedCtx, client); err != nil {
			_ = client.Close()
			return nil, nil, err
		}
	}

	return client, drv.DB(), nil
}
