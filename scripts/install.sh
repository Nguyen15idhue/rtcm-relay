#!/usr/bin/env bash
# install.sh — Cai dat rtcm-relay len VPS Linux tu source (yeu cau CGO/libpcap)
# Chay tren VPS: bash scripts/install.sh
# Hoac full setup: curl -sSL https://raw.githubusercontent.com/USER/REPO/master/scripts/install.sh | sudo bash
set -e

INSTALL_DIR="/opt/rtcm-relay"
SERVICE_DST="/etc/systemd/system/rtcm-relay.service"
GO_VERSION="1.22.3"
GO_ARCHIVE="go${GO_VERSION}.linux-amd64.tar.gz"
GO_URL="https://go.dev/dl/${GO_ARCHIVE}"

# Mau sac
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[+]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
die()  { echo -e "${RED}[x]${NC} $1"; exit 1; }

# Kiem tra root
[ "$(id -u)" -eq 0 ] || die "Can chay voi quyen root: sudo bash scripts/install.sh"

log "=== RTCM Relay Installer ==="

# 1. Cai cac goi he thong can thiet
log "Cai dat libpcap-dev, git, curl..."
apt-get update -qq
apt-get install -y libpcap-dev git curl build-essential

# 2. Kiem tra / cai Go
if ! command -v go &>/dev/null; then
    warn "Go chua duoc cai. Dang cai Go ${GO_VERSION}..."
    curl -sSL "$GO_URL" -o "/tmp/${GO_ARCHIVE}"
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "/tmp/${GO_ARCHIVE}"
    rm "/tmp/${GO_ARCHIVE}"
    export PATH="$PATH:/usr/local/go/bin"
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
    log "Da cai Go ${GO_VERSION} tai /usr/local/go"
else
    log "Go da san sang: $(go version)"
export PATH="$PATH:/usr/local/go/bin"
fi

# 3. Tao thu muc
mkdir -p "$INSTALL_DIR"

# 4. Clone hoac pull code
if [ -d "$INSTALL_DIR/.git" ]; then
    log "Cap nhat code tu git..."
    cd "$INSTALL_DIR" && git pull
else
    # Neu chua co code, check xem co san trong thu muc hien tai khong
    if [ -f "$(pwd)/go.mod" ] && grep -q 'rtcm-relay' "$(pwd)/go.mod" 2>/dev/null; then
        log "Dung code tu thu muc hien tai..."
        cp -r . "$INSTALL_DIR/"
        cd "$INSTALL_DIR"
    else
        die "Khong tim thay source code. Chay script nay tu thu muc goc cua project."
    fi
fi

# 5. Build
log "Dang build rtcm-relay..."
cd "$INSTALL_DIR"
PATH="$PATH:/usr/local/go/bin" CGO_ENABLED=1 go build -o rtcm-relay ./cmd/main.go
chmod +x rtcm-relay
log "Build thanh cong: $INSTALL_DIR/rtcm-relay"

# 6. Copy config neu chua co
if [ ! -f "$INSTALL_DIR/config.yaml" ]; then
    warn "Chua co config.yaml. Kiem tra lai thu muc va tao file config.yaml."
else
    log "config.yaml san sang tai $INSTALL_DIR/config.yaml"
    warn "Hay kiem tra: interface (mac dinh 'auto'), port, destination host"
fi

# 7. Cai systemd service
if [ -f "$INSTALL_DIR/deploy/rtcm-relay.service" ]; then
    cp "$INSTALL_DIR/deploy/rtcm-relay.service" "$SERVICE_DST"
    systemctl daemon-reload
    systemctl enable rtcm-relay
    log "Da cai va enable systemd service"
fi

echo ""
echo "========================================"
echo "Cai dat xong. Cac lenh quan ly:"
echo ""
echo "  sudo systemctl start   rtcm-relay"
echo "  sudo systemctl stop    rtcm-relay"
echo "  sudo systemctl restart rtcm-relay"
echo "  sudo systemctl status  rtcm-relay"
echo "  sudo journalctl -u rtcm-relay -f    # Xem log realtime"
echo "========================================"
echo ""
warn "Neu interface la 'auto', service se tu phat hien network interface."
warn "De chi dinh thu cong: chinh sua $INSTALL_DIR/config.yaml => interface: \"eth0\""
