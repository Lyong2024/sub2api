package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// cliOptions holds process flags. Explicitly provided flags are applied as
// environment variables (higher priority than .env / defaults).
type cliOptions struct {
	showHelp    bool
	showVersion bool
	setupCLI    bool
	runServer   bool // true when user passed enough to start the service

	// config / paths
	configFile string
	dataDir    string

	// topology
	deployMode    string
	runMode       string
	autoSetup     string // "true"/"false"/""
	dbDriver      string
	dbPath        string
	redisEmbedded string // "true"/"false"/""

	// server
	host     string
	port     string
	timezone string

	// bootstrap admin / secrets
	adminEmail    string
	adminPassword string
	jwtSecret     string
}

func exeName() string {
	base := filepath.Base(os.Args[0])
	if base == "" || base == "." {
		return "sub2api"
	}
	return base
}

func printUsage(w io.Writer) {
	name := exeName()
	fmt.Fprintf(w, `Sub2API %s  (commit %s)

单二进制 API 网关。请通过命令行参数启动；直接双击/无参数运行只会显示本帮助。

用法:
  %s [选项]

────────────────────────────────────────
常用启动方式
────────────────────────────────────────

  # 1) DIY 模式（SQLite + 内嵌 Redis，推荐个人/单机）
  %s -deploy-mode=diy -auto-setup ^
      -admin-email=admin@example.com -admin-password=YourStrongPass

  # 2) 加载 YAML 配置文件（路径可用 -config / -c）
  %s -config=.\config.yaml
  %s -c /etc/sub2api/config.yaml

  # 3) DIY + 指定数据目录与库文件
  %s -deploy-mode=diy -data-dir=.\data -db-path=.\data\sub2api.db -auto-setup

  # 4) 标准模式（外部 PostgreSQL + Redis，需先配好 config.yaml）
  %s -deploy-mode=standard -config=.\config.yaml

  # 5) 交互式安装向导（终端问答，非 Web）
  %s -setup

  # 6) 版本
  %s -version

Windows PowerShell 示例（一行）:
  .\%s -deploy-mode=diy -auto-setup -admin-email=admin@example.com -admin-password=pass123456 -port=8080

Linux / macOS:
  ./%s -deploy-mode=diy -auto-setup -admin-email=admin@example.com -admin-password=pass123456

浏览器: http://127.0.0.1:8080
首次 DIY 自动安装会写入 config.yaml、.installed，并创建管理员（密码见控制台或 -admin-password）。

────────────────────────────────────────
选项
────────────────────────────────────────
  -h, -help, --help     显示本帮助
  -version              显示版本号

  -config, -c <path>    指定 YAML 配置文件（设置 CONFIG_FILE）
  -data-dir <path>      数据/配置目录（DATA_DIR）

  -deploy-mode <mode>   部署拓扑: diy | standard
                        diy      = SQLite WAL + 内嵌 Redis，默认 8080
                        standard = PostgreSQL + 外部 Redis
  -run-mode <mode>      运行模式: standard | simple
                        simple   = 跳过余额/计费校验（内网调试）

  -auto-setup           无安装锁时自动初始化（DIY 推荐；等价 AUTO_SETUP=true）
  -no-auto-setup        禁用自动初始化，走 Web/CLI 安装向导

  -db-driver <driver>   数据库驱动: sqlite | postgres
  -db-path <path>       SQLite 文件路径（DATABASE_PATH），默认 ./data/sub2api.db
  -redis-embedded       启用进程内 Redis（DIY 默认）
  -no-redis-embedded    使用外部 Redis（读 config/环境中的 REDIS_*）

  -host <addr>          监听地址（SERVER_HOST），默认 0.0.0.0
  -port <port>          监听端口（SERVER_PORT），默认 8080
  -timezone, -tz <iana> 时区，如 Asia/Shanghai 或 UTC

  -admin-email <email>  首次安装管理员邮箱（ADMIN_EMAIL）
  -admin-password <pwd> 首次安装管理员密码（ADMIN_PASSWORD；空则自动生成并打印）
  -jwt-secret <secret>  JWT 密钥，建议 ≥32 字节（JWT_SECRET）

  -setup                运行终端交互式安装向导后退出

────────────────────────────────────────
配置优先级（高 → 低）
────────────────────────────────────────
  1. 命令行参数
  2. 进程环境变量
  3. 工作目录 .env / DATA_DIR/.env
  4. -config 指定的 YAML 或默认搜索的 config.yaml
  5. 内置默认值（本 DIY 发行版默认 deploy_mode=diy）

YAML 示例见包内 config.example.yaml；环境变量示例见 .env.example。
公开注册默认关闭，请用管理员登录后在后台开启。

`, Version, Commit, name, name, name, name, name, name, name, name, name, name)
}

// parseCLI parses os.Args. Returns options and whether the process should exit
// after handling (help/version already printed by caller when needed).
func parseCLI(args []string) (*cliOptions, error) {
	opt := &cliOptions{}

	// No arguments → friendly help only (do not start server).
	if len(args) <= 1 {
		opt.showHelp = true
		return opt, nil
	}

	// Bare help tokens
	for _, a := range args[1:] {
		switch strings.ToLower(a) {
		case "-h", "-help", "--help", "help", "/?":
			opt.showHelp = true
			return opt, nil
		}
	}

	fs := flag.NewFlagSet(exeName(), flag.ContinueOnError)
	fs.SetOutput(io.Discard) // we print our own usage

	var (
		help          bool
		version       bool
		setupCLI      bool
		autoSetup     bool
		noAutoSetup   bool
		redisEmb      bool
		noRedisEmb    bool
		configPath    string
		configPathShort string
		dataDir       string
		deployMode    string
		runMode       string
		dbDriver      string
		dbPath        string
		host          string
		port          string
		timezone      string
		tzShort       string
		adminEmail    string
		adminPassword string
		jwtSecret     string
	)

	fs.BoolVar(&help, "help", false, "show help")
	fs.BoolVar(&help, "h", false, "show help")
	fs.BoolVar(&version, "version", false, "show version")
	fs.BoolVar(&setupCLI, "setup", false, "run interactive CLI setup wizard")

	fs.StringVar(&configPath, "config", "", "path to config.yaml")
	fs.StringVar(&configPathShort, "c", "", "path to config.yaml (short)")
	fs.StringVar(&dataDir, "data-dir", "", "data directory (DATA_DIR)")

	fs.StringVar(&deployMode, "deploy-mode", "", "diy|standard")
	fs.StringVar(&runMode, "run-mode", "", "standard|simple")

	fs.BoolVar(&autoSetup, "auto-setup", false, "enable auto setup on first run")
	fs.BoolVar(&noAutoSetup, "no-auto-setup", false, "disable auto setup")

	fs.StringVar(&dbDriver, "db-driver", "", "sqlite|postgres")
	fs.StringVar(&dbPath, "db-path", "", "sqlite database file path")

	fs.BoolVar(&redisEmb, "redis-embedded", false, "use in-process Redis")
	fs.BoolVar(&noRedisEmb, "no-redis-embedded", false, "use external Redis")

	fs.StringVar(&host, "host", "", "listen host")
	fs.StringVar(&port, "port", "", "listen port")
	fs.StringVar(&timezone, "timezone", "", "IANA timezone")
	fs.StringVar(&tzShort, "tz", "", "IANA timezone (short)")

	fs.StringVar(&adminEmail, "admin-email", "", "bootstrap admin email")
	fs.StringVar(&adminPassword, "admin-password", "", "bootstrap admin password")
	fs.StringVar(&jwtSecret, "jwt-secret", "", "JWT secret")

	if err := fs.Parse(args[1:]); err != nil {
		return nil, err
	}

	if help {
		opt.showHelp = true
		return opt, nil
	}
	opt.showVersion = version
	opt.setupCLI = setupCLI

	if configPathShort != "" {
		configPath = configPathShort
	}
	if tzShort != "" {
		timezone = tzShort
	}

	opt.configFile = strings.TrimSpace(configPath)
	opt.dataDir = strings.TrimSpace(dataDir)
	opt.deployMode = strings.TrimSpace(deployMode)
	opt.runMode = strings.TrimSpace(runMode)
	opt.dbDriver = strings.TrimSpace(dbDriver)
	opt.dbPath = strings.TrimSpace(dbPath)
	opt.host = strings.TrimSpace(host)
	opt.port = strings.TrimSpace(port)
	opt.timezone = strings.TrimSpace(timezone)
	opt.adminEmail = strings.TrimSpace(adminEmail)
	opt.adminPassword = adminPassword // allow spaces; do not trim middle
	opt.jwtSecret = strings.TrimSpace(jwtSecret)

	if autoSetup {
		opt.autoSetup = "true"
	}
	if noAutoSetup {
		opt.autoSetup = "false"
	}
	if redisEmb {
		opt.redisEmbedded = "true"
	}
	if noRedisEmb {
		opt.redisEmbedded = "false"
	}

	// Starting the server unless pure help/version (setup is a separate path).
	if !version {
		opt.runServer = true
	}
	// -setup alone runs wizard then exits (still "handled")
	if setupCLI {
		opt.runServer = false
	}

	return opt, nil
}

// applyCLIToEnv maps explicitly provided CLI flags to environment variables.
// Call after LoadDotEnvFiles so flags win over .env.
func applyCLIToEnv(opt *cliOptions) {
	if opt == nil {
		return
	}
	set := func(key, val string) {
		if strings.TrimSpace(val) == "" {
			return
		}
		_ = os.Setenv(key, val)
	}
	// password/secret may be intentionally empty-skipped; only set if non-empty after trim for email etc.
	set("CONFIG_FILE", opt.configFile)
	set("DATA_DIR", opt.dataDir)
	set("DEPLOY_MODE", opt.deployMode)
	set("RUN_MODE", opt.runMode)
	set("AUTO_SETUP", opt.autoSetup)
	set("DATABASE_DRIVER", opt.dbDriver)
	set("DATABASE_PATH", opt.dbPath)
	set("REDIS_EMBEDDED", opt.redisEmbedded)
	set("SERVER_HOST", opt.host)
	set("SERVER_PORT", opt.port)
	set("TZ", opt.timezone)
	set("TIMEZONE", opt.timezone)
	set("ADMIN_EMAIL", opt.adminEmail)
	if opt.adminPassword != "" {
		_ = os.Setenv("ADMIN_PASSWORD", opt.adminPassword)
	}
	set("JWT_SECRET", opt.jwtSecret)
}
