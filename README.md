# Node Exporter Custom

<div align="center">

![Version](https://img.shields.io/badge/version-${{ steps.get_tag.outputs.version }}-blue.svg)

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

1. Скачайте релиз NITRINOnetControlManager.exe и два файла install-service.bat и uninstall-service.bat из [раздела Releases](https://github.com/yourusername/yourrepository/releases).
2. Поместите эти файлы в одну папку на вашем компьютере.
3. Запустите install-service.bat от имени администратора, чтобы установить службу.

### Удаление службы

1. Запустите uninstall-service.bat от имени администратора, чтобы удалить службу.
