#!/usr/bin/env bash
set -euo pipefail

# Путь к бинарю можно передать первым аргументом, иначе ./nitrinonetcmanager
BIN="${1:-./nitrinonetcmanager}"

API_PASSWORD="ys51Bi3P5OSIS48"

CONFIG_DIR="/etc/nitrinonetcmanager"
CERT_DIR="$CONFIG_DIR/certs"
LOG_DIR="/var/log/nitrinonetcmanager"
STATE_DIR="/var/lib/nitrinonetcmanager"
ENV_FILE="$CONFIG_DIR/ncm.env"
PID_FILE="$STATE_DIR/ncm.pid"
SERVICE_LOG="$LOG_DIR/service.log"

if [[ ! -f "$BIN" ]]; then
  echo "Не найден бинарь: $BIN"
  echo "Использование: bash setup_ncm.sh /путь/к/nitrinonetcmanager"
  exit 1
fi
if [[ ! -x "$BIN" ]]; then
  chmod +x "$BIN"
fi

# Устанавливаем бинарь в стандартное место, чтобы ncmctl работал без аргументов
sudo cp "$BIN" /usr/local/bin/nitrinonetcmanager
sudo chmod +x /usr/local/bin/nitrinonetcmanager

echo "[1/8] Установка зависимостей"
sudo apt-get update -y
sudo apt-get install -y openssl smartmontools nvme-cli dmidecode pciutils lsof

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
echo "$API_PASSWORD" | sudo tee "$CONFIG_DIR/api.password" >/dev/null

echo "[5/8] Генерация handshake ключа (ослаблю права для удобства curl)"
# Для отладки сделаем 0644, чтобы текущий пользователь мог читать ключ.
sudo sh -c "openssl rand -base64 32 > '$CONFIG_DIR/handshake.key'"
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
sudo tee "$ENV_FILE" >/dev/null <<EOF
NCM_API_PASSWORD_FILE=$CONFIG_DIR/api.password
NCM_HANDSHAKE_KEY_FILE=$CONFIG_DIR/handshake.key
NCM_CERT_DIR=$CERT_DIR
NCM_LOG_DIR=$LOG_DIR
NCM_STATE_DIR=$STATE_DIR
EOF
sudo chmod 644 "$ENV_FILE"

echo "[8/8] Запуск агента с корректным окружением и создание ncmctl"
# Запускаем в фоне, пишем PID и лог
sudo bash -c "set -a; source '$ENV_FILE'; set +a; nohup '/usr/local/bin/nitrinonetcmanager' > '$SERVICE_LOG' 2>&1 & echo \$! > '$PID_FILE'"

# Утилита управления
sudo tee /usr/local/bin/ncmctl >/dev/null <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

CONFIG_DIR="/etc/nitrinonetcmanager"
ENV_FILE="$CONFIG_DIR/ncm.env"
LOG_DIR="/var/log/nitrinonetcmanager"
STATE_DIR="/var/lib/nitrinonetcmanager"
PID_FILE="$STATE_DIR/ncm.pid"
SERVICE_LOG="$LOG_DIR/service.log"
BIN_DEFAULT="/usr/local/bin/nitrinonetcmanager"

cmd="${1:-status}"
bin="${2:-$BIN_DEFAULT}"

exists() { command -v "$1" >/dev/null 2>&1; }

start() {
  if [[ ! -f "$bin" ]]; then
    echo "Бинарь не найден: $bin"
    exit 1
  fi
  mkdir -p "$LOG_DIR" "$STATE_DIR"
  if [[ -f "$PID_FILE" ]] && ps -p "$(cat "$PID_FILE")" >/dev/null 2>&1; then
    echo "Уже запущен (PID $(cat "$PID_FILE"))"
    exit 0
  fi
  # Гасим процессы на портах
  PIDS=$(lsof -t -i :9182 -i :9183 || true)
  [[ -n "$PIDS" ]] && kill $PIDS || true

  bash -c "set -a; source '$ENV_FILE'; set +a; nohup '$bin' > '$SERVICE_LOG' 2>&1 & echo \$! > '$PID_FILE'"
  echo "Запущен (PID $(cat "$PID_FILE"))"
}

stop() {
  if [[ -f "$PID_FILE" ]]; then
    PID=$(cat "$PID_FILE")
    kill "$PID" || true
    sleep 1
    ps -p "$PID" >/dev/null 2>&1 && kill -9 "$PID" || true
    rm -f "$PID_FILE"
    echo "Остановлен"
  else
    echo "PID-файл не найден; пытаюсь освободить порты"
    PIDS=$(lsof -t -i :9182 -i :9183 || true)
    [[ -n "$PIDS" ]] && kill $PIDS || true
  fi
}

status() {
  if [[ -f "$PID_FILE" ]] && ps -p "$(cat "$PID_FILE")" >/dev/null 2>&1; then
    echo "Статус: запущен (PID $(cat "$PID_FILE"))"
  else
    echo "Статус: не запущен"
  fi
  echo "Порты:"
  ss -lntp | grep -E '9182|9183' || true
}

case "$cmd" in
  start) start ;;
  stop) stop ;;
  restart) stop; start ;;
  status) status ;;
  *) echo "Использование: ncmctl {start|stop|restart|status} [путь/к/бинарю]"; exit 1 ;;
esac
EOF
sudo chmod +x /usr/local/bin/ncmctl

echo "Готово."
echo "Управление: sudo ncmctl {start|stop|restart|status} [/путь/к/бинарю]"
echo "Лог: $SERVICE_LOG"
echo "PID: $PID_FILE"