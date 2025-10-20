#!/usr/bin/env bash
set -euo pipefail

# Путь к бинарю можно передать первым аргументом, иначе ./nitrinonetcmanager
BIN="${1:-./nitrinonetcmanager}"
# Необязательный второй аргумент: общий handshake-ключ для всех агентов (если не задан - читаем из конфигурации или генерируем)
HANDSHAKE="${2:-}"
# Необязательный третий аргумент: путь к внешнему конфигу установщика (если не задан - автопоиск)
CONFIG_FILE="${3:-}"

API_PASSWORD_DEFAULT="ys51Bi3P5OSIS48"

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
  API_PASSWORD="${NCM_API_PASSWORD:-$API_PASSWORD_DEFAULT}"
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

# Прочитать внешний конфиг (если указан/найден)
read_installer_conf

# Устанавливаем бинарь в стандартное место, чтобы ncmctl работал без аргументов
sudo cp "$BIN" /usr/local/bin/nitrinonetcmanager
sudo chmod +x /usr/local/bin/nitrinonetcmanager

# Установка зависимостей кросс-дистрибутивно
install_prereqs

echo "[2/8] Остановка процессов, занявших порты 9182/9183"
PIDS=$(sudo lsof -t -i :9182 -i :9183 || true)
if [[ -n "${PIDS}" ]]; then
  echo "Нашёл процессы: ${PIDS}. Посылаю SIGTERM..."
  sudo kill ${PIDS} || true
  sleep 1
  # Если кто-то ещё жив — добиваю
  SURVIVORS=$(sudo lsof -t -i :9182 -i :9183 || true)
  if [[ -n "${SURVIVORS}" ]]; then
    echo "Процессы всё ещё держат порт, посылаю SIGKILL: ${SURVIVORS}"
    sudo kill -9 ${SURVIVORS} || true
  fi
fi

echo "[3/8] Создание каталогов"
sudo mkdir -p "$CONFIG_DIR" "$CERT_DIR" "$LOG_DIR" "$STATE_DIR"

echo "[4/8] Запись API-пароля"
echo "${API_PASSWORD}" | sudo tee "$CONFIG_DIR/api.password" >/dev/null

echo "[5/8] Генерация/задание handshake ключа"
if [[ -n "$HANDSHAKE" ]]; then
  echo "$HANDSHAKE" | sudo tee "$CONFIG_DIR/handshake.key" >/dev/null
else
  if [[ -n "${NCM_HANDSHAKE_KEY:-}" ]]; then
    echo "$NCM_HANDSHAKE_KEY" | sudo tee "$CONFIG_DIR/handshake.key" >/dev/null
  else
    sudo sh -c "openssl rand -base64 32 > '$CONFIG_DIR/handshake.key'"
  fi
fi
sudo chmod 644 "$CONFIG_DIR/handshake.key"

echo "[6/8] Генерация самоподписанного сертификата (c SAN)"
HOST="$(hostname)"
sudo openssl req -x509 -newkey rsa:2048 \
  -keyout "$CERT_DIR/key.pem" \
  -out "$CERT_DIR/cert.pem" \
  -days 365 -nodes \
  -subj "/CN=${HOST}" \
  -addext "subjectAltName=DNS:${HOST},DNS:localhost,IP:127.0.0.1"

echo "[7/8] Создание файла окружения"
# после записи ENV-файла
sudo tee "$ENV_FILE" >/dev/null <<EOF
NCM_API_PASSWORD_FILE=$CONFIG_DIR/api.password
NCM_HANDSHAKE_KEY_FILE=$CONFIG_DIR/handshake.key
NCM_CERT_DIR=$CERT_DIR
NCM_LOG_FILE=$LOG_DIR/service.log
NCM_STATE_DIR=$STATE_DIR
EOF
sudo chmod 644 "$ENV_FILE"

# Создаём systemd unit и включаем автозапуск, если systemd доступен
write_systemd_unit
if command -v systemctl >/dev/null 2>&1; then
  sudo systemctl enable --now nitrinonetcmanager || true
fi

echo "[8/8] Запуск агента с корректным окружением и создание ncmctl"
# блок [8/8] — запуск через nohup только если НЕТ systemd
echo "[8/8] Запуск агента с корректным окружением и создание ncmctl"
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