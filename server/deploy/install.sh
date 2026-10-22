#!/usr/bin/env bash
# RuleGo Server 一键安装（Linux 裸机，非 Docker）。
#
# 远程安装：
#   curl -fsSL https://raw.githubusercontent.com/rulego/rulego/main/server/deploy/install.sh | bash
# 本地/离线安装：
#   bash install.sh --offline rulego-server-all-linux-amd64.tar.gz
# 常用参数：--version vX.Y.Z 指定版本 | --port N 监听端口 | --dir PATH 安装目录 |
#           --variant all|standard 组件档 | --uninstall 卸载
# 国内镜像：RULEGO_MIRROR=https://<ghproxy镜像>/ bash install.sh
#
# 布局：二进制+editor+config.conf 装在 <dir>（root 默认 /opt/rulego-server，非 root 默认
# ~/.rulego-server；WorkingDirectory 即此，data/ 等相对路径全部落在 <dir>/data，零配置改动）。
# systemd 服务用户 rulego（仅 root 安装时创建）。升级 = 重跑脚本：二进制与 editor 覆盖，
# 已存在的 config.conf 保留（新版存为 config.conf.new 供比对），data/ 不动。
set -u

REPO="rulego/rulego"
DEFAULT_DIR="/opt/rulego-server"
BIN_NAME="rulego-server"
SERVICE_NAME="rulego-server"

VERSION=""
INSTALL_DIR=""
PORT=""
VARIANT="all"
OFFLINE_FILE=""
UNINSTALL=0
NON_INTERACTIVE=0

log()  { printf '\033[32m[install]\033[0m %s\n' "$*"; }
warn() { printf '\033[33m[install]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[31m[install]\033[0m %s\n' "$*" >&2; exit 1; }

usage() {
  sed -n '2,16p' "$0" 2>/dev/null
  exit 0
}

while [ $# -gt 0 ]; do
  case "$1" in
    --version)    VERSION="${2:?}"; shift 2 ;;
    --dir)        INSTALL_DIR="${2:?}"; shift 2 ;;
    --port)       PORT="${2:?}"; shift 2 ;;
    --variant)    VARIANT="${2:?}"; shift 2 ;;
    --offline)    OFFLINE_FILE="${2:?}"; shift 2 ;;
    --uninstall)  UNINSTALL=1; shift ;;
    --yes)        NON_INTERACTIVE=1; shift ;;
    -h|--help)    usage ;;
    *) die "未知参数: $1（--help 查看用法）" ;;
  esac
done

[ "$(uname -s)" = "Linux" ] || die "本脚本仅支持 Linux；Windows/macOS 请下载对应 release 包手动安装，或使用 Docker"

if [ -z "$INSTALL_DIR" ]; then
  if [ "$(id -u)" -eq 0 ]; then INSTALL_DIR="$DEFAULT_DIR"; else INSTALL_DIR="$HOME/.rulego-server"; fi
fi

# ---------- 卸载 ----------
if [ "$UNINSTALL" -eq 1 ]; then
  if [ ! -d "$INSTALL_DIR" ] && [ ! -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
    die "未发现已安装的 ${SERVICE_NAME}"
  fi
  if [ "$NON_INTERACTIVE" -ne 1 ]; then
    # curl|bash 时 stdin 是脚本自身，确认必须走终端
    if [ -r /dev/tty ]; then
      printf '将停止服务并删除 %s（含 data/ 全部数据）。确认请输入 yes：' "$INSTALL_DIR"
      read -r ANSWER < /dev/tty || die "无法读取确认输入（非交互环境请加 --yes）"
      [ "$ANSWER" = "yes" ] || die "已取消"
    else
      die "非交互环境无法确认删除，如确认请加 --yes"
    fi
  fi
  if command -v systemctl >/dev/null 2>&1; then
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    systemctl disable "$SERVICE_NAME" 2>/dev/null || true
    rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
    systemctl daemon-reload 2>/dev/null || true
  fi
  rm -rf "$INSTALL_DIR"
  # 只清理本脚本创建的用户（home 指向安装目录才算数），避免误删同名既有账号
  if id rulego >/dev/null 2>&1; then
    USER_HOME="$(getent passwd rulego | cut -d: -f6)"
    [ "$USER_HOME" = "$INSTALL_DIR" ] && userdel rulego 2>/dev/null || true
  fi
  log "已卸载 ${SERVICE_NAME}"
  exit 0
fi

# ---------- 架构与下载 ----------
case "$(uname -m)" in
  x86_64)          ARCH="amd64" ;;
  aarch64|arm64)   ARCH="arm64" ;;
  i686|i386)       ARCH="386" ;;
  *) die "不支持的架构: $(uname -m)（armv7 请用 Docker 镜像）" ;;
esac

if [ "$VARIANT" != "all" ] && [ "$VARIANT" != "standard" ]; then
  die "--variant 只支持 all|standard"
fi
if [ "$VARIANT" = "all" ]; then
  ASSET="rulego-server-all-linux-${ARCH}.tar.gz"
else
  ASSET="rulego-server-linux-${ARCH}.tar.gz"
fi

fetch() {
  # fetch <url> <输出文件>：curl/wget 任一可用即可；RULEGO_MIRROR 作为 URL 前缀（国内加速）
  _url="${RULEGO_MIRROR:-}$1"
  _out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fSL --retry 3 -o "$_out" "$_url"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$_out" "$_url"
  else
    die "需要 curl 或 wget 之一"
  fi
}

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

if [ -n "$OFFLINE_FILE" ]; then
  [ -f "$OFFLINE_FILE" ] || die "离线包不存在: $OFFLINE_FILE"
  log "离线安装: $OFFLINE_FILE"
  cp "$OFFLINE_FILE" "$TMP_DIR/$ASSET"
else
  if [ -n "$VERSION" ]; then
    BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
  else
    log "获取最新版本"
    BASE_URL="https://github.com/${REPO}/releases/latest/download"
  fi
  log "下载 ${ASSET}"
  fetch "${BASE_URL}/${ASSET}" "$TMP_DIR/$ASSET" || die "下载失败（网络问题或该版本不存在；国内可用 RULEGO_MIRROR=<镜像前缀> 重试）"
  # 校验和尽力而为：sha256sums.txt 下载失败不阻断安装
  if fetch "${BASE_URL}/sha256sums.txt" "$TMP_DIR/sha256sums.txt" 2>/dev/null; then
    (cd "$TMP_DIR" && grep "$ASSET" sha256sums.txt | sha256sum -c - >/dev/null 2>&1) \
      && log "校验和通过" || warn "校验和校验失败或不可用，继续安装"
  fi
fi

# ---------- 安装 ----------
log "安装到 ${INSTALL_DIR}"
mkdir -p "$INSTALL_DIR" || die "无法创建安装目录: $INSTALL_DIR"

# 升级保护：先暂存已有 config.conf，解压后换回（归档新版配置留作 config.conf.new 供比对）
RESTORE_CONFIG=0
if [ -f "${INSTALL_DIR}/config.conf" ]; then
  mv "${INSTALL_DIR}/config.conf" "${INSTALL_DIR}/.config.conf.keep"
  RESTORE_CONFIG=1
fi

tar -xzf "$TMP_DIR/$ASSET" -C "$INSTALL_DIR" || die "解压失败"

if [ "$RESTORE_CONFIG" -eq 1 ]; then
  mv "${INSTALL_DIR}/config.conf" "${INSTALL_DIR}/config.conf.new"
  mv "${INSTALL_DIR}/.config.conf.keep" "${INSTALL_DIR}/config.conf"
  log "保留已有 config.conf，归档新版在 config.conf.new"
fi

# 归档内二进制带平台后缀（rulego-server-all-linux-amd64 等），统一改名。
# 必须无条件覆盖：升级重装时 rulego-server 已存在，跳过 mv 会让 systemd 继续跑旧二进制
RENAMED=0
for f in "${INSTALL_DIR}"/rulego-server-*; do
  [ -e "$f" ] || continue
  [ "$f" = "${INSTALL_DIR}/${BIN_NAME}" ] && continue
  mv -f "$f" "${INSTALL_DIR}/${BIN_NAME}" && RENAMED=1
  break
done
if [ "$RENAMED" -eq 1 ]; then
  chmod +x "${INSTALL_DIR}/${BIN_NAME}"
elif [ ! -f "${INSTALL_DIR}/${BIN_NAME}" ]; then
  die "找不到发布包内的二进制"
fi

if [ -n "$PORT" ]; then
  sed -i "s|^server = .*|server = :${PORT}|" "${INSTALL_DIR}/config.conf"
  log "监听端口已设为 ${PORT}"
fi
PORT="${PORT:-9090}"

# ---------- systemd 服务（仅 root）----------
if [ "$(id -u)" -eq 0 ]; then
  if ! id rulego >/dev/null 2>&1; then
    NOLOGIN_SHELL="/bin/false"
    for _s in /usr/sbin/nologin /sbin/nologin /bin/false; do
      [ -x "$_s" ] && NOLOGIN_SHELL="$_s" && break
    done
    useradd --system --home-dir "$INSTALL_DIR" --shell "$NOLOGIN_SHELL" rulego 2>/dev/null \
      || warn "创建系统用户 rulego 失败，服务将以 root 运行"
  fi
  chown -R rulego:rulego "$INSTALL_DIR" 2>/dev/null || chown -R rulego "$INSTALL_DIR" 2>/dev/null || true

  if [ -d /run/systemd/system ]; then
    log "写入 systemd 服务 ${SERVICE_NAME}"
    # 加固说明：ProtectSystem=strict 只放行 ReadWritePaths 内的写路径。规则链里的
    # exec/file 节点若要写其他目录（如业务数据盘），把目录追加到 ReadWritePaths 并
    # systemctl daemon-reload。
    cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=RuleGo Server
Documentation=https://github.com/${REPO}
After=network-online.target
Wants=network-online.target

# 防重启风暴：5 分钟内超过 5 次重启则停止并置 failed（留现场供排查）。
# 这两项必须在 [Unit] 段，放 [Service] 会被 systemd 忽略。
StartLimitIntervalSec=300
StartLimitBurst=5

[Service]
Type=simple
User=rulego
Group=rulego
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/${BIN_NAME} -c ${INSTALL_DIR}/config.conf
# 用户可用 KEY=VALUE 覆盖配置（RULEGO_* 环境变量或 JWT_SECRET_KEY 等）
EnvironmentFile=-/etc/default/${SERVICE_NAME}

Restart=always
RestartSec=5s
KillSignal=SIGTERM
TimeoutStopSec=20s

# 最小权限加固
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=${INSTALL_DIR} /tmp
ProtectKernelTunables=true
ProtectControlGroups=true
RestrictSUIDSGID=true

StandardOutput=journal
StandardError=journal
SyslogIdentifier=${SERVICE_NAME}

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable --now "$SERVICE_NAME"
    log "等待服务启动"
    sleep 3
    if command -v curl >/dev/null 2>&1; then
      curl -fsS "http://127.0.0.1:${PORT}/api/v1/ping" >/dev/null 2>&1 \
        && log "服务健康检查通过 (http://127.0.0.1:${PORT}/api/v1/ping)" \
        || warn "健康检查未通过，查看日志: journalctl -u ${SERVICE_NAME} -n 50"
    fi
  else
    warn "未检测到 systemd，手动启动: cd ${INSTALL_DIR} && ./${BIN_NAME}"
  fi
else
  warn "非 root 安装到 ${INSTALL_DIR}（无 systemd 服务），启动: cd ${INSTALL_DIR} && ./${BIN_NAME}"
fi

INSTALLED_VERSION="$("${INSTALL_DIR}/${BIN_NAME}" -v 2>/dev/null || echo unknown)"
log "安装完成: ${INSTALLED_VERSION}"
log "控制台: http://<本机IP>:${PORT}/editor/ （默认账号 admin/admin，生产环境请改密或开启 require_auth）"
