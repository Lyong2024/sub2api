#!/usr/bin/env bash
# Install Sub2API DIY as a systemd service on Linux.
# Usage (as root):
#   ./install-systemd.sh /path/to/sub2api
set -euo pipefail

BIN_SRC="${1:-./sub2api}"
INSTALL_DIR="${INSTALL_DIR:-/opt/sub2api}"
DATA_DIR="${DATA_DIR:-/var/lib/sub2api}"
ENV_FILE="${ENV_FILE:-/etc/sub2api/sub2api.env}"
SERVICE_USER="${SERVICE_USER:-sub2api}"

if [[ "$(id -u)" -ne 0 ]]; then
  echo "Please run as root (sudo)." >&2
  exit 1
fi

if [[ ! -f "$BIN_SRC" ]]; then
  echo "Binary not found: $BIN_SRC" >&2
  exit 1
fi

id -u "$SERVICE_USER" &>/dev/null || useradd --system --home "$DATA_DIR" --shell /usr/sbin/nologin "$SERVICE_USER"

mkdir -p "$INSTALL_DIR" "$DATA_DIR" "$(dirname "$ENV_FILE")"
install -m 0755 "$BIN_SRC" "$INSTALL_DIR/sub2api"

if [[ ! -f "$ENV_FILE" ]]; then
  cat >"$ENV_FILE" <<EOF
DEPLOY_MODE=diy
AUTO_SETUP=true
DATA_DIR=$DATA_DIR
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
# ADMIN_EMAIL=admin@example.com
# ADMIN_PASSWORD=change-me
# JWT_SECRET=change-this-to-a-secure-random-string-32b
TZ=Asia/Shanghai
EOF
  chmod 0600 "$ENV_FILE"
  echo "Wrote $ENV_FILE — edit secrets before first start if needed."
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "$SCRIPT_DIR/sub2api.service" ]]; then
  install -m 0644 "$SCRIPT_DIR/sub2api.service" /etc/systemd/system/sub2api.service
else
  echo "sub2api.service not found next to installer; writing a minimal unit."
  cat >/etc/systemd/system/sub2api.service <<'UNIT'
[Unit]
Description=Sub2API DIY
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=sub2api
WorkingDirectory=/opt/sub2api
EnvironmentFile=-/etc/sub2api/sub2api.env
Environment=DEPLOY_MODE=diy
Environment=DATA_DIR=/var/lib/sub2api
ExecStart=/opt/sub2api/sub2api
Restart=on-failure
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
UNIT
fi

chown -R "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR" "$INSTALL_DIR"

systemctl daemon-reload
systemctl enable sub2api.service
systemctl restart sub2api.service
systemctl --no-pager --full status sub2api.service || true

echo
echo "Sub2API DIY installed."
echo "  Binary : $INSTALL_DIR/sub2api"
echo "  Data   : $DATA_DIR"
echo "  Env    : $ENV_FILE"
echo "  Service: systemctl status sub2api"
echo "  UI     : http://SERVER_IP:8080"
