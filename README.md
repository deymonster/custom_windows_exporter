# Node Exporter Custom

<div align="center">

![Version](https://img.shields.io/badge/version-v1.0.0-blue.svg)

</div>

## 📋 Описание

Node Exporter Custom — это мониторинговый агент для сбора различных системных метрик на Windows. Он предоставляет данные в формате Prometheus и предназначен для интеграции с Prometheus для удобного мониторинга ресурсов на вашем сервере.

## 🔍 Собираемые метрики

### 💻 Системная информация

- system_information
  - Имя ПК
  - Версия ОС
  - Архитектура
  - Производитель
  - Модель

### 🔲 Процессор (CPU)

- cpu_usage_percent: Загрузка процессора по ядрам в процентах
- cpu_temperature_celsius: Температура процессора (если доступна)

### 🎮 Оперативная память (RAM)

- total_memory_bytes: Общий объем памяти
- used_memory_bytes: Объем использованной памяти
- free_memory_bytes: Объем свободной памяти
- memory_module_info: Информация о модулях памяти
  - Производитель
  - Номер партии
  - Серийный номер

### 💽 Дисковая подсистема

- disk_usage_bytes: Использование пространства на каждом логическом диске
- disk_usage_percent: Процент использования дисков
- disk_read_bytes_per_second: Скорость чтения на диске
- disk_write_bytes_per_second: Скорость записи на диске
- disk_health_status: Статус здоровья каждого физического диска

### 🌐 Сетевые интерфейсы

- network_status: Статус сетевого подключения
- network_rx_bytes_per_second: Входящая пропускная способность сети
- network_tx_bytes_per_second: Исходящая пропускная способность сети
- network_errors: Количество ошибок на интерфейсе
- network_dropped_packets: Количество отброшенных пакетов

### 🎮 Видеокарта

- gpu_info: Название и модель видеокарты
- gpu_memory_total_bytes: Общий объем памяти видеокарты

### 🔧 Материнская плата

- baseboard_info: Информация о материнской плате
  - Производитель
  - Продукт
  - Версия



## 🚀 Установка

### Установка службы

1. Скачайте MSI-установщик NITRINOnetControlManager.msi из [раздела Releases](https://github.com/yourusername/yourrepository/releases).
2. Запустите установщик от имени администратора, чтобы установить службу Node Exporter Custom.
3. Во время установки автоматически будет открыт порт 9183 для доступа к собранным метрикам.

После завершения установки служба начнет собирать системные метрики и предоставлять их по указанному порту для интеграции с Prometheus.

### Сборка и установка Linux-версии

Поддерживается полноценная сборка агента под Linux. Кратко процесс выглядит так:

```bash
go mod tidy
GOOS=linux GOARCH=amd64 go build -o bin/nitrinonetcmanager ./service
```

> ℹ️ **В PowerShell** команды экспорта переменных отличаются: 
> ```powershell
> $env:GOOS = 'linux'
> $env:GOARCH = 'amd64'
> go build -o bin/nitrinonetcmanager ./service
> ```
> В `cmd.exe` используйте `set GOOS=linux` и `set GOARCH=amd64` перед вызовом `go build`.

Более подробное руководство по развёртыванию, настройке переменных окружения и интеграции с systemd доступно в [docs/linux_build.md](docs/linux_build.md).

### Удаление службы

1. Для удаления службы запустите NITRINOnetControlManager.msi снова или воспользуйтесь Панелью управления Windows.

**Linux: деплой и запуск**

- Установка через скрипт (копирует бинарь в `/usr/local/bin/nitrinonetcmanager`, генерирует секреты и запускает агент):
```bash
bash ./setup_ncm.sh ./bin/nitrinonetcmanager
```

- Управление агентом:
```bash
sudo ncmctl {start|stop|restart|status|uninstall}
```

- Проверка метрик (порт `9182`), требуется заголовок `X-Agent-Handshake-Key`:
```bash
curl -H "X-Agent-Handshake-Key: <ВАШ_КЛЮЧ>" http://<HOST>:9182/metrics
```

**Сборка для ARM64 (при необходимости)**

- Если целевая машина — ARM (например, Raspberry Pi 4, AWS Graviton):
```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/nitrinonetcmanager ./service
```

### Настройка Handshake Key (Linux)

- По умолчанию `setup_ncm.sh` генерирует ключ и записывает его в файл `NCM_HANDSHAKE_KEY_FILE=/etc/nitrinonetcmanager/handshake.key`.
- Чтобы задать единый ключ для всех агентов, передайте его вторым аргументом при установке:
```bash
bash ./setup_ncm.sh ./bin/nitrinonetcmanager "<ЕДИНЫЙ_ШАРЕНЫЙ_КЛЮЧ>"
```
- Ключ читается менеджером секретов из переменных окружения:
  - `NCM_HANDSHAKE_KEY_FILE` — путь к файлу с ключом (рекомендовано, поддерживается hot-reload при замене содержимого)
  - или `NCM_HANDSHAKE_KEY` — ключ напрямую из окружения (меняется только при перезапуске процесса)
- Метрики доступны только при наличии правильного заголовка:
```bash
curl -H "X-Agent-Handshake-Key: <ЕДИНЫЙ_ШАРЕНЫЙ_КЛЮЧ>" http://<HOST>:9182/metrics
```

### Обновление UUID после замены железа (Linux)

- Агент хранит текущий UUID в `NCM_STATE_DIR/hardware_uuid` (по умолчанию `/var/lib/nitrinonetcmanager/hardware_uuid`).
- Если состав железа изменился, метрика `UNIQUE_ID_CHANGED` станет `1`. Чтобы подтвердить изменения и записать новый UUID:
```bash
curl -k -u "<ТЕКУЩИЙ_UUID>:<API_ПАРОЛЬ>" https://<HOST>:9183/api/update-uuid
```
- Аутентификация:
  - Логин: текущий UUID (считывается из `/var/lib/nitrinonetcmanager/hardware_uuid`)
  - Пароль: берётся из `NCM_API_PASSWORD_FILE` (по умолчанию `/etc/nitrinonetcmanager/api.password`)
- После успешного вызова:
  - файл `hardware_uuid` обновляется
  - `UNIQUE_ID_CHANGED` становится `0`
  - `UNIQUE_ID_SYSTEM{uuid="..."} 1` отражает новый UUID

### Конфигурация секретов (Linux)

- API-пароль:
  - `NCM_API_PASSWORD_FILE=/etc/nitrinonetcmanager/api.password`
  - альтернативно: `NCM_API_PASSWORD` или `NCM_API_PASSWORD_HASH`/`NCM_API_PASSWORD_HASH_FILE` (SHA-256)
- Handshake Key:
  - `NCM_HANDSHAKE_KEY_FILE=/etc/nitrinonetcmanager/handshake.key`
  - альтернативно: `NCM_HANDSHAKE_KEY`
- Директория состояния:
  - `NCM_STATE_DIR` (по умолчанию `/var/lib/nitrinonetcmanager`)
  - хранит `hardware_uuid` и `ncm.pid`

### Удаление агента (Linux)

- Полная деинсталляция (останавливает процесс и удаляет бинарь, конфиги, логи, state):
```bash
sudo ncmctl uninstall
```
- После удаления можно повторно выполнить установку:
```bash
bash ./setup_ncm.sh ./bin/nitrinonetcmanager "<ЕДИНЫЙ_ШАРЕНЫЙ_КЛЮЧ>"
```

### Устранение неполадок (Linux)

- “Бинарь не найден”: скопируйте бинарь в ожидаемый путь и перезапустите:
```bash
sudo cp ./bin/nitrinonetcmanager /usr/local/bin/nitrinonetcmanager && sudo chmod +x /usr/local/bin/nitrinonetcmanager && sudo ncmctl restart
```
- Проверить лог сервиса:
```bash
sudo tail -n 100 /var/log/nitrinonetcmanager/service.log
```
