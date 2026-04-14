#!/bin/bash
#
# CheckUser — installer
# Uso: bash <(curl -sL https://raw.githubusercontent.com/DemonHunter29/CheckUser/master/install.sh)
#

REPO="DemonHunter29/CheckUser"
SERVICE_NAME="checkuser"
BINARY_PATH="/usr/local/bin/${SERVICE_NAME}"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
DEFAULT_PORT="2052"

get_arch() {
    case "$(uname -m)" in
        x86_64 | x64 | amd64)      echo 'amd64' ;;
        armv8 | arm64 | aarch64)   echo 'arm64' ;;
        armv7l | armv7 | arm)      echo 'arm' ;;
        *)                         echo 'unsupported' ;;
    esac
}

latest_tag() {
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep -Po '"tag_name"\s*:\s*"\K[^"]+' \
        | head -n1
}

download_binary() {
    local arch=$(get_arch)
    if [ "$arch" = "unsupported" ]; then
        echo -e "\e[1;31mArquitetura de CPU não suportada: $(uname -m)\e[0m"
        return 1
    fi

    local tag=$(latest_tag)
    if [ -z "$tag" ]; then
        echo -e "\e[1;31mNão foi possível descobrir a release mais recente.\e[0m"
        return 1
    fi

    local name="checkuser-linux-${arch}"
    local url="https://github.com/${REPO}/releases/download/${tag}/${name}"

    echo -e "⬇️  Baixando \e[1;36m${name}\e[0m (${tag})..."
    # -4 força IPv4 (alguns edges CDN do GH retornam "Not Found" via IPv6).
    # -L segue redirects (releases do GH redirecionam pra objects.githubusercontent.com).
    # -A força UA de browser — alguns CDNs do GH bloqueiam UA padrão de curl.
    # --retry 10 cobre transient 404/timeout logo após uma release ser publicada.
    # NÃO usa --retry-all-errors (não existe em curl antigo do Debian 10/Ubuntu 18).
    if ! curl -4 -L -A "Mozilla/5.0" --retry 10 --retry-delay 3 \
            -o "$BINARY_PATH" "$url"; then
        echo -e "\e[1;31mFalha no download de ${url}\e[0m"
        return 1
    fi
    # Sanidade — se o GH devolveu HTML de erro, o arquivo terá <100KB.
    local size
    size=$(stat -c%s "$BINARY_PATH" 2>/dev/null || echo 0)
    if [ "$size" -lt 100000 ]; then
        echo -e "\e[1;31mDownload corrompido ($size bytes). Tente novamente em 30s.\e[0m"
        rm -f "$BINARY_PATH"
        return 1
    fi
    chmod +x "$BINARY_PATH"
}

check_url_access() {
    local test_url=$1
    echo -e "\n🔍 Testando acesso externo a: $test_url"
    if curl -s --max-time 5 "$test_url" >/dev/null; then
        echo -e "\e[1;32m✅ A URL está acessível externamente.\e[0m"
        return
    fi

    echo -e "\e[1;31m❌ Não foi possível acessar a URL externamente.\e[0m"
    echo -ne "\e[1;33mDeseja abrir a porta no iptables automaticamente? [s/N]: \e[0m"
    read answer

    if [[ "$answer" =~ ^[Ss]$ ]]; then
        local port=$(echo "$test_url" | grep -oE ':[0-9]+' | tr -d ':')
        sudo iptables -I INPUT -p tcp --dport "$port" -j ACCEPT
        sudo iptables-save > /etc/iptables.rules 2>/dev/null || true
        echo -e "\e[1;32m✔ Porta $port liberada no iptables.\e[0m"
        return
    fi

    echo -e "\e[1;33m⚠ Porta não foi aberta. Faça isso manualmente se necessário.\e[0m"
}

install_service() {
    download_binary || exit 1

    local port=$DEFAULT_PORT
    local sslEnabled=""
    local proto="http"
    local addr=$(curl -s https://ipv4.icanhazip.com)

    if systemctl is-active --quiet "$SERVICE_NAME"; then
        echo "🛑 Parando serviço $SERVICE_NAME..."
        systemctl stop "$SERVICE_NAME"
        systemctl disable "$SERVICE_NAME" &>/dev/null
    fi

    cat << EOF | tee "$SERVICE_FILE" > /dev/null
[Unit]
Description=CheckUser Service
After=network.target nss-lookup.target

[Service]
User=root
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
NoNewPrivileges=true
ExecStart=${BINARY_PATH} --start --port ${port} ${sslEnabled}
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload &>/dev/null
    systemctl enable "$SERVICE_NAME" &>/dev/null
    systemctl start "$SERVICE_NAME"

    local final_url="${proto}://${addr}:${port}"
    echo -e "\n\e[1;32m✅ CheckUser instalado com sucesso!\e[0m"
    echo -e "\e[1;34m🌐 URL: \e[1;36m${final_url}\e[0m"

    check_url_access "$final_url"

    echo -e "\nPressione Enter para continuar..."
    read
}

reinstall_service() {
    echo "♻️  Reinstalando CheckUser..."
    systemctl stop "$SERVICE_NAME" &>/dev/null
    systemctl disable "$SERVICE_NAME" &>/dev/null
    rm -f "$BINARY_PATH" "$SERVICE_FILE"
    systemctl daemon-reload &>/dev/null
    install_service
}

uninstall_service() {
    echo "🧹 Desinstalando CheckUser..."
    systemctl stop "$SERVICE_NAME" &>/dev/null
    systemctl disable "$SERVICE_NAME" &>/dev/null
    rm -f "$BINARY_PATH" "$SERVICE_FILE"
    systemctl daemon-reload &>/dev/null
    echo -e "\e[1;31m✔ CheckUser removido.\e[0m"
    echo -e "\nPressione Enter para continuar..."
    read
}

main() {
    clear
    echo '---------------------------------'
    echo -ne '     \e[1;33mCHECKUSER\e[0m'
    if [[ -x "$BINARY_PATH" ]]; then
        echo -e ' \e[1;32mv'$("$BINARY_PATH" --version | awk '{print $2}')'\e[0m'
    else
        echo -e ' \e[1;31m[DESINSTALADO]\e[0m'
    fi
    echo '---------------------------------'
    echo -e '\e[1;32m[01] - \e[1;31mINSTALAR CHECKUSER\e[0m'
    echo -e '\e[1;32m[02] - \e[1;31mREINSTALAR CHECKUSER\e[0m'
    echo -e '\e[1;32m[03] - \e[1;31mDESINSTALAR CHECKUSER\e[0m'
    echo -e '\e[1;32m[00] - \e[1;31mSAIR\e[0m'
    echo '---------------------------------'
    echo -ne '\e[1;32mEscolha uma opção: \e[0m'
    read option

    case $option in
        1) install_service; main ;;
        2) reinstall_service; main ;;
        3) uninstall_service; main ;;
        0) echo "Saindo." ;;
        *) echo -e "\e[1;31mOpção inválida. Tente novamente.\e[0m"; read; main ;;
    esac
}

if [[ $EUID -ne 0 ]]; then
    echo "Execute como root (sudo)."
    exit 1
fi

main
