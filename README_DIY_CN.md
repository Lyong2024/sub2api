# Sub2API DIY 模式

单二进制独立运行：**内嵌 Web UI + SQLite（WAL）+ 进程内 Redis**。  
只需暴露 **8080** 端口，无需外部 PostgreSQL / Redis。

> English: [README_DIY.md](README_DIY.md)

## 能力一览

| 项目 | DIY |
|------|-----|
| 数据库 | SQLite3 WAL（`data/sub2api.db`） |
| 缓存 / 锁 | 进程内 miniredis |
| 前端 | 编译进二进制（`-tags embed`） |
| 端口 | `8080`（API + 管理界面） |
| 配置 | `.env` 和/或 `config.yaml` |
| 首次安装 | `AUTO_SETUP=true`（DIY 默认开启） |

## 下载预编译二进制

**每次推送到 `diy` 分支**（以及 `diy-v*` 标签）会触发 GitHub Actions 自动构建多平台包。

1. 打开仓库 **Actions** → 工作流 **DIY Release**
2. 下载对应系统架构的 Artifact，例如 `sub2api-diy-linux-amd64`，解压得到 `sub2api`
3. 或在 `diy-v*` Release 附件中下载

支持：`linux/amd64`、`linux/arm64`、`windows/amd64`、`darwin/amd64`、`darwin/arm64`。

## 快速启动（命令行参数，推荐）

**直接双击 / 无参数运行**只会打印帮助，不会启动服务。请带参数运行：

```bash
# 查看帮助
./sub2api -h          # Windows: sub2api.exe -h

# DIY 首次安装并启动（SQLite + 内嵌 Redis）
./sub2api -deploy-mode=diy -auto-setup \
  -admin-email=admin@example.com \
  -admin-password=请改成强密码 \
  -port=8080

# 使用 YAML 配置启动
./sub2api -config=./config.yaml
# 简写
./sub2api -c ./config.yaml

# 指定数据目录与库文件
./sub2api -deploy-mode=diy -data-dir=./data -db-path=./data/sub2api.db -auto-setup

# 标准模式（PostgreSQL + 外部 Redis，需先写好 config.yaml）
./sub2api -deploy-mode=standard -config=./config.yaml

# 终端交互安装向导
./sub2api -setup
```

Windows PowerShell：

```powershell
.\sub2api.exe -deploy-mode=diy -auto-setup -admin-email=admin@example.com -admin-password=pass123456 -port=8080
.\sub2api.exe -c .\config.yaml
```

浏览器打开 `http://127.0.0.1:8080`。  
首次 DIY 自动安装会：建库（WAL）、创建管理员、写入 `config.yaml` 与 `.installed`。

### Chrome 打开却显示「以前本地项目」的页面？

`127.0.0.1:8080` / `localhost:8080` 常被多个本地项目共用。若以前某个前端注册过 **Service Worker**，Chrome 可能一直劫持该源，显示旧站而不是 Sub2API。

处理（任选其一）：

1. **推荐**：Chrome 打开 `chrome://settings/content/all` → 搜索 `127.0.0.1` → 删除该站点数据  
2. 或 F12 → Application → Service Workers → Unregister；Storage → Clear site data  
3. 或用 **无痕模式** / 换端口：`sub2api.exe -port=18080` 后访问 `http://127.0.0.1:18080`  
4. 硬刷新：`Ctrl+Shift+R`（有时不够，需清站点数据）

新版本会在 HTML 中主动 `unregister` Service Worker，并对常见 `/sw.js` 路径返回 410。

### 配置优先级（高 → 低）

1. **命令行参数**
2. 进程环境变量
3. 工作目录 `.env` / `DATA_DIR/.env`
4. `-config` 指定的 YAML 或默认搜索的 `config.yaml`
5. 内置默认值（本 DIY 发行版默认 `deploy_mode=diy`）

### 常用参数

| 参数 | 含义 |
|------|------|
| `-h` / `-help` | 帮助 |
| `-config` / `-c` | YAML 配置文件路径 |
| `-deploy-mode` | `diy` 或 `standard` |
| `-run-mode` | `standard` 或 `simple`（跳过计费） |
| `-auto-setup` / `-no-auto-setup` | 首次自动安装 |
| `-db-path` | SQLite 路径 |
| `-data-dir` | 数据目录 |
| `-host` / `-port` | 监听地址端口 |
| `-admin-email` / `-admin-password` | 首次管理员 |
| `-jwt-secret` | JWT 密钥 |
| `-timezone` / `-tz` | 时区，如 `Asia/Shanghai` |
| `-setup` | 交互式安装向导 |
| `-version` | 版本 |

### 环境变量（可选，等价于参数）

| 变量 | 含义 |
|------|------|
| `DEPLOY_MODE` | `diy` / `standard` |
| `DATABASE_PATH` | SQLite 路径 |
| `AUTO_SETUP` | 自动安装 |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | 初始管理员 |
| `CONFIG_FILE` | 配置文件路径 |
| `SERVER_PORT` | 端口 |
| `DATA_DIR` | 数据目录 |

仍可用 `.env` 或 `config.example.yaml` → `config.yaml`，但**推荐参数启动**，避免 bat/sh 包装。

## Linux：systemd 开机自启

发布包中包含 `sub2api.service` 与 `install-systemd.sh`。

### 一键安装

```bash
sudo ./install-systemd.sh ./sub2api
```

### 手动安装

```bash
sudo useradd --system --home /var/lib/sub2api --shell /usr/sbin/nologin sub2api || true
sudo mkdir -p /opt/sub2api /var/lib/sub2api /etc/sub2api
sudo install -m 0755 ./sub2api /opt/sub2api/sub2api
sudo cp deploy/diy/sub2api.service /etc/systemd/system/sub2api.service
sudo cp deploy/diy/.env.example /etc/sub2api/sub2api.env
sudo nano /etc/sub2api/sub2api.env   # 设置 ADMIN_PASSWORD、JWT_SECRET
sudo chown -R sub2api:sub2api /var/lib/sub2api /opt/sub2api

sudo systemctl daemon-reload
sudo systemctl enable --now sub2api
sudo systemctl status sub2api
```

常用运维命令：

```bash
# 看日志
sudo journalctl -u sub2api -f

# 重启 / 停止
sudo systemctl restart sub2api
sudo systemctl stop sub2api

# 开机禁用
sudo systemctl disable sub2api
```

使用 unit 文件时，数据目录为 `/var/lib/sub2api`（`DATA_DIR`）。

### 可选：反向代理与 HTTPS

用 Caddy / Nginx 终结 TLS，反代到本机 `127.0.0.1:8080` 即可。  
应用本身只需要 8080 这一个进程端口。

## Windows 快速运行

```powershell
$env:DEPLOY_MODE = "diy"
$env:AUTO_SETUP = "true"
.\sub2api.exe
```

开机自启可使用「任务计划程序」在登录/启动时运行，或用 NSSM 注册为 Windows 服务，环境变量与 Linux 相同。

## 从源码编译

```bash
cd frontend && pnpm install && pnpm run build && cd ..
cd backend
CGO_ENABLED=0 go build -tags embed -ldflags="-s -w" -o ../dist/sub2api ./cmd/server

# 或
make diy-build
make diy-run
```

## 数据与备份

| 路径 | 说明 |
|------|------|
| `sub2api.db` | 主库 |
| `sub2api.db-wal` / `-shm` | WAL 附属文件（备份需一起拷贝） |
| `config.yaml` | 运行配置（含密钥） |
| `.installed` | 安装锁 |

冷备建议先停服务：

```bash
sudo systemctl stop sub2api
sudo tar czf sub2api-backup.tgz -C /var/lib/sub2api .
sudo systemctl start sub2api
```

## 限制说明

- **单进程**：不适合多实例水平扩展
- **SQLite 写串行**：适合中小规模
- **Ops / 重度聚合**：默认关闭（`ops.enabled=false`）
- Outbox 的 `FOR UPDATE SKIP LOCKED` 路径在 DIY 下跳过，由周期全量重建 + 进程内缓存覆盖
- 大规模生产请使用 standard 模式（PostgreSQL + Redis）

## 故障排查

| 现象 | 处理 |
|------|------|
| 端口占用 | 改 `SERVER_PORT` 或释放 8080 |
| 反复进入安装 | 勿随意删除 `.installed` 与 `config.yaml` |
| 无法登录 | 使用新 `DATA_DIR` 重装，或从备份恢复数据库 |
| systemd 权限错误 | `chown -R sub2api:sub2api /var/lib/sub2api` |

## 许可证

与主项目 Sub2API 相同。
