#!/usr/bin/env bash
set -euo pipefail

# Путь к бинарю можно передать первым аргументом, иначе ./nitrinonetcmanager
BIN="${1:-./nitrinonetcmanager}"
# Необязательный второй аргумент: общий handshake-ключ для всех агентов (если не задан - читаем из конфигурации или генерируем)
HANDSHAKE="${2:-}"
# Необязательный третий аргумент: путь к внешнему конфигу установщика (если не задан - автопоиск)
CONFIG_FILE="${3:-}"

CONFIG_DIR="/etc/nitrinonetcmanager"
CERT_DIR="$CONFIG_DIR/certs"
LOG_DIR="/var/log/nitrinonetcmanager"
STATE_DIR="/var/lib/nitrinonetcmanager"
ENV_FILE="$CONFIG_DIR/ncm.env"
PID_FILE="$STATE_DIR/ncm.pid"
SERVICE_LOG="$LOG_DIR/service.log"

if [[ ! -f "$BIN" ]]; then
  echo "Не найден бинарь: $BIN"
  echo "Использование: bash setup_ncm.sh /путь/к/nitrinonetcmanager [HANDSHAKE] [CONFIG_FILE]"
  exit 1
fi
if [[ ! -x "$BIN" ]]; then
  chmod +x "$BIN"
fi

# Добавляем отсутствующие функции: read_installer_conf и install_prereqs
read_installer_conf() {
  local candidate
  if [[ -n "${CONFIG_FILE:-}" && -f "$CONFIG_FILE" ]]; then
    candidate="$CONFIG_FILE"
  else
    for f in "$PWD/ncm_installer.conf" "./ncm_installer.conf" "/etc/nitrinonetcmanager/ncm_installer.conf"; do
      if [[ -f "$f" ]]; then
        candidate="$f"
        break
      fi
    done
  fi

  if [[ -n "${candidate:-}" ]]; then
    # shellcheck disable=SC1090
    source "$candidate"
  fi

  # Устанавливаем переменные с дефолтами, если их нет
  # Do not ship a shared API password. A caller may supply one through the
  # installer config; otherwise generate a unique value for this host.
  API_PASSWORD="${NCM_API_PASSWORD:-}"
  if [[ -z "$API_PASSWORD" ]]; then
    API_PASSWORD="$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')"
  fi
  export API_PASSWORD

  # HANDSHAKE можно задать через аргумент 2, либо через конфиг (NCM_HANDSHAKE_KEY)
  if [[ -z "${HANDSHAKE:-}" && -n "${NCM_HANDSHAKE_KEY:-}" ]]; then
    HANDSHAKE="$NCM_HANDSHAKE_KEY"
  fi
}

install_prereqs() {
  local pkgs=(openssl lsof)
  if command -v apt-get >/dev/null 2>&1; then
    sudo apt-get update -y
    sudo apt-get install -y "${pkgs[@]}"
  elif command -v dnf >/dev/null 2>&1; then
    sudo dnf install -y "${pkgs[@]}"
  elif command -v yum >/dev/null 2>&1; then
    sudo yum install -y "${pkgs[@]}"
  elif command -v zypper >/dev/null 2>&1; then
    sudo zypper install -y "${pkgs[@]}"
  elif command -v apk >/dev/null 2>&1; then
    sudo apk add --no-cache "${pkgs[@]}"
  else
    echo "Предупреждение: пакетный менеджер не найден; пропускаю установку зависимостей."
  fi
}

configure_firewall() {
  # Do not expose metrics to the whole network by default. The panel address
  # (or a managed subnet) is supplied by the one-line install command.
  local allowed="${NCM_ALLOWED_CIDRS:-}"
  if [[ -z "$allowed" ]]; then
    echo "[7/9] Firewall: not changed (set NCM_ALLOWED_CIDRS to allow the panel to reach port 9182)"
    return
  fi

  local source
  allowed="${allowed//,/ }"
  for source in $allowed; do
    if [[ ! "$source" =~ ^[0-9A-Fa-f:.]+(/[0-9]{1,3})?$ ]]; then
      echo "Invalid NCM_ALLOWED_CIDRS entry: $source" >&2
      exit 1
    fi

    if command -v ufw >/dev/null 2>&1 && sudo ufw status | grep -q '^Status: active'; then
      sudo ufw allow from "$source" to any port 9182 proto tcp >/dev/null
      echo "[7/9] Firewall: UFW allows $source -> 9182/tcp"
      continue
    fi

    if command -v firewall-cmd >/dev/null 2>&1 && sudo firewall-cmd --state >/dev/null 2>&1; then
      local family="ipv4"
      [[ "$source" == *:* ]] && family="ipv6"
      sudo firewall-cmd --permanent --add-rich-rule="rule family=\"$family\" source address=\"$source\" port port=\"9182\" protocol=\"tcp\" accept" >/dev/null
      sudo firewall-cmd --reload >/dev/null
      echo "[7/9] Firewall: firewalld allows $source -> 9182/tcp"
      continue
    fi

    echo "[7/9] Firewall: no active UFW/firewalld detected; verify port 9182/tcp is reachable from $source"
  done
}

write_systemd_unit() {
  if command -v systemctl >/dev/null 2>&1; then
    sudo tee /etc/systemd/system/nitrinonetcmanager.service >/dev/null <<'EOF'
[Unit]
Description=NITRINO NetC Manager
After=network.target
Requires=network-online.target
After=network-online.target

[Service]
Type=simple
WorkingDirectory=/etc/nitrinonetcmanager
EnvironmentFile=/etc/nitrinonetcmanager/ncm.env
ExecStartPre=/bin/bash -lc 'pids=$(lsof -t -i :9182 -i :9183 2>/dev/null || true); for pid in $pids; do if ps -o comm= -p "$pid" 2>/dev/null | grep -qx "nitrinonetcmanager"; then kill "$pid" || true; sleep 1; kill -9 "$pid" || true; fi; done'
ExecStart=/usr/local/bin/nitrinonetcmanager
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
EOF
    sudo systemctl daemon-reload
  fi
}

stop_existing_agent() {
  if command -v systemctl >/dev/null 2>&1 && sudo systemctl is-active --quiet nitrinonetcmanager; then
    echo "[1/8] Остановка существующей службы nitrinonetcmanager"
    sudo systemctl stop nitrinonetcmanager
  fi

  # Do not kill arbitrary applications merely because they use a monitoring
  # port. A different owner is a deployment error that needs attention.
  local listener
  listener="$(sudo lsof -t -iTCP:9182 -sTCP:LISTEN 2>/dev/null || true)"
  if [[ -n "$listener" ]]; then
    echo "Порт 9182 всё ещё занят процессом: $listener" >&2
    echo "Остановите конфликтующую службу и повторите установку." >&2
    exit 1
  fi
}

# Прочитать внешний конфиг (если указан/найден)
read_installer_conf

# Установка зависимостей кросс-дистрибутивно
install_prereqs

# On upgrade the executable is in use by the running service. Stop it before
# copying the replacement; otherwise Linux returns ETXTBSY (Text file busy).
stop_existing_agent

# Устанавливаем бинарь в стандартное место, чтобы ncmctl работал без аргументов
sudo cp "$BIN" /usr/local/bin/nitrinonetcmanager
sudo chmod +x /usr/local/bin/nitrinonetcmanager

echo "[2/9] Создание каталогов"
sudo mkdir -p "$CONFIG_DIR" "$CERT_DIR" "$LOG_DIR" "$STATE_DIR"

echo "[3/9] Запись API-пароля"
echo "${API_PASSWORD}" | sudo tee "$CONFIG_DIR/api.password" >/dev/null
sudo chmod 600 "$CONFIG_DIR/api.password"

echo "[4/9] Генерация/задание handshake ключа"
if [[ -n "$HANDSHAKE" ]]; then
  echo "$HANDSHAKE" | sudo tee "$CONFIG_DIR/handshake.key" >/dev/null
else
  if [[ -n "${NCM_HANDSHAKE_KEY:-}" ]]; then
    echo "$NCM_HANDSHAKE_KEY" | sudo tee "$CONFIG_DIR/handshake.key" >/dev/null
  else
    sudo sh -c "openssl rand -base64 32 > '$CONFIG_DIR/handshake.key'"
  fi
fi
sudo chmod 600 "$CONFIG_DIR/handshake.key"

echo "[5/9] Генерация самоподписанного сертификата (c SAN)"
HOST="$(hostname)"
sudo openssl req -x509 -newkey rsa:2048 \
  -keyout "$CERT_DIR/key.pem" \
  -out "$CERT_DIR/cert.pem" \
  -days 365 -nodes \
  -subj "/CN=${HOST}" \
  -addext "subjectAltName=DNS:${HOST},DNS:localhost,IP:127.0.0.1"
sudo chmod 600 "$CERT_DIR/key.pem"
sudo chmod 644 "$CERT_DIR/cert.pem"

echo "[6/9] Создание файла окружения"
# после записи ENV-файла
sudo tee "$ENV_FILE" >/dev/null <<EOF
NCM_API_PASSWORD_FILE=$CONFIG_DIR/api.password
NCM_HANDSHAKE_KEY_FILE=$CONFIG_DIR/handshake.key
NCM_PROFILE=${NCM_PROFILE:-auto}
NCM_ALLOWED_CIDRS=${NCM_ALLOWED_CIDRS:-}
NCM_CERT_DIR=$CERT_DIR
NCM_LOG_FILE=$LOG_DIR/service.log
NCM_STATE_DIR=$STATE_DIR
EOF
sudo chmod 600 "$ENV_FILE"

configure_firewall

# Создаём systemd unit и включаем автозапуск, если systemd доступен
write_systemd_unit
if command -v systemctl >/dev/null 2>&1; then
  sudo systemctl enable --now nitrinonetcmanager || true
fi

echo "[8/9] Запуск агента с корректным окружением и создание ncmctl"
# блок [8/8] — запуск через nohup только если НЕТ systemd
echo "[8/9] Запуск агента с корректным окружением и создание ncmctl"
if ! command -v systemctl >/dev/null 2>&1; then
  sudo bash -c "set -a; source '$ENV_FILE'; set +a; nohup '/usr/local/bin/nitrinonetcmanager' > '$SERVICE_LOG' 2>&1 & echo \$! > '$PID_FILE'"
fi

# Утилита управления
sudo tee /usr/local/bin/ncmctl >/dev/null <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

SERVICE="nitrinonetcmanager"
CONFIG_DIR="/etc/nitrinonetcmanager"
ENV_FILE="$CONFIG_DIR/ncm.env"
LOG_DIR="/var/log/nitrinonetcmanager"
STATE_DIR="/var/lib/nitrinonetcmanager"
PID_FILE="$STATE_DIR/ncm.pid"
BIN_DEFAULT="/usr/local/bin/nitrinonetcmanager"

cmd="${1:-status}"
bin="${2:-$BIN_DEFAULT}"

has_systemd() { command -v systemctl >/dev/null 2>&1; }

start() {
  if has_systemd; then
    sudo systemctl enable --now "$SERVICE"
  else
    bash -c "set -a; source '$ENV_FILE'; set +a; nohup '$bin' >> '$LOG_DIR/service.log' 2>&1 & echo $! > '$PID_FILE'"
  fi
  echo "Запущен."
}

stop() {
  if has_systemd; then
    sudo systemctl stop "$SERVICE" || true
    sudo systemctl disable "$SERVICE" || true
  else
    if [[ -f "$PID_FILE" ]]; then
      kill "$(cat "$PID_FILE")" || true
      rm -f "$PID_FILE"
    fi
  fi
  echo "Остановлен."
}

restart() {
  if has_systemd; then
    sudo systemctl restart "$SERVICE"
  else
    stop || true
    start
  fi
}

status() {
  if has_systemd; then
    sudo systemctl status "$SERVICE" --no-pager || true
  else
    if [[ -f "$PID_FILE" ]] && ps -p "$(cat "$PID_FILE")" >/dev/null 2>&1; then
      echo "Статус: запущен (PID $(cat "$PID_FILE"))"
    else
      echo "Статус: не запущен"
    fi
  fi
}

uninstall() {
  stop || true
  if has_systemd; then
    sudo rm -f "/etc/systemd/system/${SERVICE}.service"
    sudo systemctl daemon-reload
  fi
  rm -f "/usr/local/bin/nitrinonetcmanager" "/usr/local/bin/ncmctl"
  rm -rf "$CONFIG_DIR" "$LOG_DIR" "$STATE_DIR"
  echo "Удалён."
}

case "$cmd" in
  start) start ;;
  stop) stop ;;
  restart) restart ;;
  status) status ;;
  uninstall) uninstall ;;
  *) echo "Использование: ncmctl {start|stop|restart|status|uninstall} [путь/к/бинарю]"; exit 1 ;;
esac
EOF
sudo chmod +x /usr/local/bin/ncmctl

echo "Готово."
echo "Управление: sudo ncmctl {start|stop|restart|status|uninstall} [/путь/к/бинарю]"
echo "Удаление: sudo ncmctl uninstall (очистит конфиги, логи, state и бинарь)"
echo "Лог: $SERVICE_LOG"
echo "PID: $PID_FILE"
