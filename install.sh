#!/bin/bash
#
# CheckUser — installer
#
# Assume que o binário 'checkuser' já foi buildado e copiado para a VPS
# (ou está no diretório atual). Para fazer o build:
#
#   go build -ldflags="-w -s -buildid=" -trimpath -o checkuser ./src
#
# Uso:
#   sudo bash install.sh

set -e

SERVICE_NAME="checkuser"
BINARY_PATH="/usr/local/bin/${SERVICE_NAME}"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
DEFAULT_PORT="5000"

color() { echo -e "\e[1;$1m$2\e[0m"; }

check_root() {
    if [[ $EUID -ne 0 ]]; then
        color 31 "Execute como root (sudo)."
        exit 1
    fi
}

find_binary() {
    for candidate in "./checkuser" "./${SERVICE_NAME}" "$(dirname "$0")/checkuser"; do
        if [[ -x "$candidate" ]]; then
            echo "$candidate"
            return 0
        fi
    done
    return 1
}

install_binary() {
    local src
    src=$(find_binary) || true
    if [[ -z "$src" ]]; then
        color 31 "Binário 'checkuser' não encontrado no diretório atual."
        echo "Faça o build primeiro:"
        echo "  go build -ldflags=\"-w -s -buildid=\" -trimpath -o checkuser ./src"
        exit 1
    fi

    color 36 "Instalando $src → $BINARY_PATH..."
    install -m 755 "$src" "$BINARY_PATH"
}

open_port() {
    local port=$1
    if command -v ufw &>/dev/null && ufw status | grep -q active; then
        ufw allow "$port"/tcp &>/dev/null || true
    fi
    if command -v iptables &>/dev/null; then
        iptables -C INPUT -p tcp --dport "$port" -j ACCEPT &>/dev/null || \
            iptables -I INPUT -p tcp --dport "$port" -j ACCEPT
    fi
}

write_service() {
    local port=$1
    local ssl_flag=$2

    cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=CheckUser service
After=network.target nss-lookup.target

[Service]
User=root
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
NoNewPrivileges=true
ExecStart=${BINARY_PATH} --start --port ${port} ${ssl_flag}
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable "$SERVICE_NAME" &>/dev/null
    systemctl restart "$SERVICE_NAME"
}

cmd_install() {
    check_root

    local port ssl_flag
    echo -ne "Porta [${DEFAULT_PORT}]: "
    read port
    port=${port:-$DEFAULT_PORT}

    echo -ne "Habilitar SSL (cert.pem/key.pem no dir atual)? [s/N]: "
    read ans
    if [[ "$ans" =~ ^[Ss]$ ]]; then
        ssl_flag="--ssl"
    else
        ssl_flag=""
    fi

    if systemctl is-active --quiet "$SERVICE_NAME"; then
        systemctl stop "$SERVICE_NAME"
    fi

    install_binary
    open_port "$port"
    write_service "$port" "$ssl_flag"

    local addr
    addr=$(curl -s --max-time 5 https://api.ipify.org || hostname -I | awk '{print $1}')
    local proto="http"
    [[ -n "$ssl_flag" ]] && proto="https"

    color 32 "OK  Serviço '${SERVICE_NAME}' instalado."
    color 36 "URL: ${proto}://${addr}:${port}"
    systemctl status "$SERVICE_NAME" --no-pager | head -n 5
}

cmd_uninstall() {
    check_root
    systemctl stop "$SERVICE_NAME" &>/dev/null || true
    systemctl disable "$SERVICE_NAME" &>/dev/null || true
    rm -f "$SERVICE_FILE" "$BINARY_PATH"
    systemctl daemon-reload
    color 31 "OK  Serviço '${SERVICE_NAME}' removido."
}

cmd_status() {
    systemctl status "$SERVICE_NAME" --no-pager
}

menu() {
    clear
    echo "--------------------------------"
    echo "   CheckUser  -  installer"
    if [[ -x "$BINARY_PATH" ]]; then
        echo "   v$("$BINARY_PATH" --version | awk '{print $2}')"
    else
        color 31 "   [NAO INSTALADO]"
    fi
    echo "--------------------------------"
    echo " 1) Instalar / atualizar"
    echo " 2) Remover"
    echo " 3) Status"
    echo " 0) Sair"
    echo "--------------------------------"
    echo -ne "Opcao: "
    read opt

    case "$opt" in
        1) cmd_install ;;
        2) cmd_uninstall ;;
        3) cmd_status ;;
        0) exit 0 ;;
        *) color 31 "Opcao invalida." ;;
    esac
    echo
    echo "Pressione Enter..."
    read
    menu
}

case "${1:-}" in
    install)   cmd_install ;;
    uninstall) cmd_uninstall ;;
    status)    cmd_status ;;
    *)         menu ;;
esac
