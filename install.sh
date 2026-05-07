#!/bin/bash
#
# HorizonVPN — CheckUser installer & manager
# Uso: bash <(curl -sL https://raw.githubusercontent.com/DemonHunter29/CheckUser/master/install.sh)

REPO="DemonHunter29/CheckUser"
SERVICE_NAME="checkuser"
BINARY_PATH="/usr/local/bin/checkuser-core"   # binário Go
WRAPPER_PATH="/usr/local/bin/checkuser"        # wrapper: sem args → menu; com args → core
MENU_PATH="/usr/local/bin/checkuser-menu"      # script de menu (cópia deste arquivo)
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
DEFAULT_PORT="2052"

# ── Cores ─────────────────────────────────────────────────────────────────────
R='\e[1;31m' G='\e[1;32m' Y='\e[1;33m' C='\e[1;36m' W='\e[1;37m' D='\e[0;37m' N='\e[0m'

# ── Utilitários ───────────────────────────────────────────────────────────────
die()  { echo -e "${R}  ✗ $*${N}"; exit 1; }
ok()   { echo -e "${G}  ✔ $*${N}"; }
info() { echo -e "${C}  → $*${N}"; }

sep()  { echo -e "${C}══════════════════════════════════════════════${N}"; }
line() { echo -e "${D}──────────────────────────────────────────────${N}"; }

press_enter() {
    echo; echo -ne "${D}  Pressione Enter para continuar...${N}"; read -r
}

confirm() {
    echo -ne "\n  ${Y}$* [s/N]: ${N}"
    read -r _ans
    [[ "$_ans" =~ ^[Ss]$ ]]
}

# ── Arquitetura ───────────────────────────────────────────────────────────────
get_arch() {
    case "$(uname -m)" in
        x86_64|x64|amd64)    echo 'amd64' ;;
        armv8|arm64|aarch64) echo 'arm64' ;;
        armv7l|armv7|arm)    echo 'arm'   ;;
        *) echo 'unsupported' ;;
    esac
}

latest_tag() {
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep -Po '"tag_name"\s*:\s*"\K[^"]+' | head -n1
}

download_binary() {
    local arch; arch=$(get_arch)
    [[ "$arch" == "unsupported" ]] && die "Arquitetura não suportada: $(uname -m)"

    local tag; tag=$(latest_tag)
    [[ -z "$tag" ]] && die "Não foi possível obter a release mais recente."

    local name="checkuser-linux-${arch}"
    local url="https://github.com/${REPO}/releases/download/${tag}/${name}"

    info "Baixando ${name} (${tag})..."
    curl -4 -L -A "Mozilla/5.0" --retry 10 --retry-delay 3 \
        -o "$BINARY_PATH" "$url" || die "Falha no download."

    local size; size=$(stat -c%s "$BINARY_PATH" 2>/dev/null || echo 0)
    if [[ "$size" -lt 100000 ]]; then
        rm -f "$BINARY_PATH"
        die "Download corrompido (${size} bytes). Tente novamente em 30s."
    fi
    chmod +x "$BINARY_PATH"
}

# ── Informações do serviço ────────────────────────────────────────────────────
svc_active() { systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; }

get_version() {
    [[ -x "$BINARY_PATH" ]] && "$BINARY_PATH" -version 2>/dev/null | awk '{print $2}'
}

get_port() {
    grep -oP '(?<=-port )\d+' "$SERVICE_FILE" 2>/dev/null | head -n1 || echo "$DEFAULT_PORT"
}

get_ip() {
    curl -4s --max-time 4 https://ipv4.icanhazip.com 2>/dev/null \
        || hostname -I 2>/dev/null | awk '{print $1}'
}

# ── Xray: garantir statsUserOnline ───────────────────────────────────────────
fix_xray_stats_online() {
    local xray_configs=(
        "/usr/local/etc/xray/config.json"
        "/etc/xray/config.json"
        "/opt/xray/config.json"
    )

    local config_path=""
    for p in "${xray_configs[@]}"; do
        [[ -f "$p" ]] && { config_path="$p"; break; }
    done

    [[ -z "$config_path" ]] && return 0  # Xray não instalado

    info "Verificando statsUserOnline no Xray (${config_path})..."

    local result
    result=$(python3 - "$config_path" <<'PYEOF'
import json, sys
path = sys.argv[1]
try:
    with open(path) as f:
        cfg = json.load(f)
    level0 = cfg.setdefault("policy", {}).setdefault("levels", {}).setdefault("0", {})
    if level0.get("statsUserOnline") is True:
        print("ok")
        sys.exit(0)
    level0["statsUserOnline"] = True
    with open(path, "w") as f:
        json.dump(cfg, f, indent=2, ensure_ascii=False)
    print("changed")
except Exception as e:
    print("error:" + str(e))
PYEOF
)

    case "$result" in
        ok)      ok "statsUserOnline já está habilitado." ;;
        changed)
            ok "statsUserOnline habilitado em ${config_path}"
            if systemctl is-active --quiet xray 2>/dev/null; then
                systemctl restart xray &>/dev/null
                ok "Xray reiniciado para aplicar a configuração."
            fi
            ;;
        *)
            echo -e "${Y}  ⚠ Não foi possível atualizar o config do Xray: ${result}${N}"
            ;;
    esac
}

# ── iptables ──────────────────────────────────────────────────────────────────
open_port() {
    local port=$1
    info "Liberando porta ${port} no firewall..."
    iptables -I INPUT -p tcp --dport "$port" -j ACCEPT 2>/dev/null
    iptables-save > /etc/iptables.rules 2>/dev/null || true
    if command -v ufw &>/dev/null && ufw status 2>/dev/null | grep -q "Status: active"; then
        ufw allow "${port}"/tcp &>/dev/null
    fi
    ok "Porta ${port} liberada."
}

# ── Service file ──────────────────────────────────────────────────────────────
write_service() {
    local port=$1
    cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=HorizonVPN CheckUser Service
After=network.target nss-lookup.target

[Service]
User=root
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
NoNewPrivileges=true
ExecStart=${BINARY_PATH} -start -port ${port}
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
}

# ── Wrapper /usr/local/bin/checkuser ─────────────────────────────────────────
# Sem argumentos → abre o menu. Com argumentos → repassa ao binário core.
install_wrapper() {
    cat > "$WRAPPER_PATH" <<'WRAPPER'
#!/bin/bash
CORE="/usr/local/bin/checkuser-core"
MENU="/usr/local/bin/checkuser-menu"

if [[ $# -eq 0 ]]; then
    [[ $EUID -ne 0 ]] && exec sudo "$0"
    [[ -x "$MENU" ]] && exec "$MENU"
    echo "checkuser-menu não encontrado. Reinstale com:"
    echo "  bash <(curl -sL https://raw.githubusercontent.com/DemonHunter29/CheckUser/master/install.sh)"
    exit 1
fi
exec "$CORE" "$@"
WRAPPER
    chmod +x "$WRAPPER_PATH"
    ok "Wrapper instalado: ${C}checkuser${N} (sem args → menu | com args → core)"
}

# ── Instalar menu script ──────────────────────────────────────────────────────
install_menu_script() {
    local src="${BASH_SOURCE[0]:-}"
    if [[ -f "$src" && "$src" != "$MENU_PATH" && "$src" != "/dev/stdin" ]]; then
        cp "$src" "$MENU_PATH"
    else
        curl -fsSL "https://raw.githubusercontent.com/${REPO}/master/install.sh" \
            -o "$MENU_PATH" &>/dev/null || true
    fi
    chmod +x "$MENU_PATH" 2>/dev/null
    [[ -x "$MENU_PATH" ]] && ok "Menu instalado: ${C}checkuser-menu${N}"
}

# ── Instalação ────────────────────────────────────────────────────────────────
install_service() {
    clear; sep
    printf "   ${C}%-44s${N}\n" "HorizonVPN — INSTALANDO CHECKUSER"
    sep; echo

    # Migração: remover binário antigo em /usr/local/bin/checkuser se existir
    # (antes o binário ficava lá; agora é o wrapper)
    if [[ -x "$WRAPPER_PATH" ]] && file "$WRAPPER_PATH" 2>/dev/null | grep -q "ELF"; then
        info "Migrando binário antigo para checkuser-core..."
        mv "$WRAPPER_PATH" "$BINARY_PATH"
    fi

    download_binary || return 1

    local port=$DEFAULT_PORT
    [[ -f "$SERVICE_FILE" ]] && port=$(get_port)

    systemctl stop "$SERVICE_NAME" &>/dev/null
    systemctl disable "$SERVICE_NAME" &>/dev/null

    write_service "$port"
    systemctl daemon-reload &>/dev/null
    systemctl enable "$SERVICE_NAME" &>/dev/null
    systemctl start "$SERVICE_NAME"
    sleep 1

    open_port "$port"
    fix_xray_stats_online
    install_wrapper
    install_menu_script

    local ip; ip=$(get_ip)
    echo; sep
    ok "CheckUser instalado com sucesso!"
    echo -e "  ${W}URL: ${C}http://${ip}:${port}${N}"
    sep
    press_enter
}

reinstall_service() {
    clear; sep
    printf "   ${Y}%-44s${N}\n" "HorizonVPN — REINSTALANDO CHECKUSER"
    sep; echo

    info "Parando serviço..."
    systemctl stop "$SERVICE_NAME" &>/dev/null
    systemctl disable "$SERVICE_NAME" &>/dev/null
    rm -f "$BINARY_PATH" "$SERVICE_FILE"
    systemctl daemon-reload &>/dev/null
    install_service
}

uninstall_service() {
    clear; sep
    printf "   ${R}%-44s${N}\n" "DESINSTALAR CHECKUSER"
    sep; echo
    echo -e "  ${Y}Esta ação remove o CheckUser e todos os dados.${N}"
    confirm "Confirmar desinstalação?" || return

    systemctl stop "$SERVICE_NAME" &>/dev/null
    systemctl disable "$SERVICE_NAME" &>/dev/null
    rm -f "$BINARY_PATH" "$WRAPPER_PATH" "$SERVICE_FILE" "$MENU_PATH"
    systemctl daemon-reload &>/dev/null
    echo; ok "CheckUser removido com sucesso."
    press_enter
    exit 0
}

# ── Logs ──────────────────────────────────────────────────────────────────────
show_logs() {
    clear
    echo -e "${C}  Exibindo logs — Ctrl+C para sair${N}\n"
    journalctl -u "$SERVICE_NAME" -f --no-pager -n 50
}

# ── Menu de dispositivos ──────────────────────────────────────────────────────
menu_devices() {
    while true; do
        clear; sep
        printf "   ${C}%-44s${N}\n" "DISPOSITIVOS REGISTRADOS"
        sep; echo
        echo -e "  ${G}[01]${N}  Listar todos os dispositivos"
        echo -e "  ${G}[02]${N}  Listar dispositivos de um usuário"
        echo -e "  ${G}[03]${N}  Remover dispositivos de um usuário"
        echo -e "  ${R}[04]${N}  Limpar banco inteiro"
        echo -e "  ${Y}[00]${N}  Voltar"
        echo; sep
        echo -ne "  ${Y}Escolha: ${N}"
        read -r opt

        case $opt in
            1)
                clear; line
                printf "  ${C}%-44s${N}\n" "TODOS OS DISPOSITIVOS"
                line; echo
                "$BINARY_PATH" -list-all-devices 2>&1
                press_enter
                ;;
            2)
                echo -ne "\n  ${Y}Usuário: ${N}"
                read -r user
                [[ -z "$user" ]] && continue
                clear; line
                printf "  ${C}Dispositivos de: ${W}%-28s${N}\n" "$user"
                line; echo
                "$BINARY_PATH" -list-devices "$user" 2>&1
                press_enter
                ;;
            3)
                echo -ne "\n  ${Y}Usuário: ${N}"
                read -r user
                [[ -z "$user" ]] && continue
                if confirm "Remover todos os devices de ${W}${user}${N}?"; then
                    echo
                    "$BINARY_PATH" -delete-devices "$user" 2>&1
                    echo; ok "Devices de '${user}' removidos."
                fi
                press_enter
                ;;
            4)
                if confirm "${R}Apagar TODOS os devices do banco? (redefine limites)${N}"; then
                    echo
                    "$BINARY_PATH" -delete-db 2>&1
                    echo; ok "Banco de dados limpo."
                fi
                press_enter
                ;;
            0) return ;;
            *) echo -e "${R}  Opção inválida.${N}"; sleep 1 ;;
        esac
    done
}

# ── Menu principal ────────────────────────────────────────────────────────────
main() {
    while true; do
        clear

        local ver; ver=$(get_version)

        sep
        if [[ -n "$ver" ]]; then
            printf "   ${C}HorizonVPN ${W}CheckUser v%-33s${N}\n" "$ver"
        else
            printf "   ${C}HorizonVPN ${W}CheckUser ${R}%-23s${N}\n" "[NÃO INSTALADO]"
        fi
        sep

        if [[ -x "$BINARY_PATH" ]]; then
            local port; port=$(get_port)
            local ip; ip=$(get_ip)
            if svc_active; then
                echo -e "  Status : ${G}● ATIVO${N}"
            else
                echo -e "  Status : ${R}○ INATIVO${N}"
            fi
            echo -e "  Porta  : ${W}${port}${N}"
            echo -e "  URL    : ${C}http://${ip}:${port}${N}"
        fi

        echo; sep

        if [[ -x "$BINARY_PATH" ]]; then
            echo -e "  ${G}[01]${N}  Atualizar CheckUser"
            echo -e "  ${G}[02]${N}  Reiniciar serviço"
            echo -e "  ${G}[03]${N}  Ver logs em tempo real"
            echo -e "  ${G}[04]${N}  Dispositivos registrados  ▶"
            echo -e "  ${R}[05]${N}  Desinstalar"
        else
            echo -e "  ${G}[01]${N}  Instalar CheckUser"
        fi
        echo -e "  ${Y}[00]${N}  Sair"
        echo; sep
        echo -ne "  ${Y}Escolha uma opção: ${N}"
        read -r option

        if [[ -x "$BINARY_PATH" ]]; then
            case $option in
                1) reinstall_service ;;
                2)
                    systemctl restart "$SERVICE_NAME"
                    echo; ok "Serviço reiniciado."
                    sleep 1
                    ;;
                3) show_logs ;;
                4) menu_devices ;;
                5) uninstall_service ;;
                0) echo "Saindo."; exit 0 ;;
                *) echo -e "${R}  Opção inválida.${N}"; sleep 1 ;;
            esac
        else
            case $option in
                1) install_service ;;
                0) echo "Saindo."; exit 0 ;;
                *) echo -e "${R}  Opção inválida.${N}"; sleep 1 ;;
            esac
        fi
    done
}

# ── Entrypoint ────────────────────────────────────────────────────────────────
[[ $EUID -ne 0 ]] && die "Execute como root (sudo)."
main
