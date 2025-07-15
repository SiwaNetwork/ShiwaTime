# ShiwaTime - Time Synchronization Software

ShiwaTime — это приложение для синхронизации времени, написанное на Go, которое повторяет функционал Timebeat. Поддерживает множество протоколов синхронизации времени и предоставляет мониторинг через Elasticsearch.

## Особенности

### Протоколы синхронизации времени
- **IEEE-1588 PTP** (Precision Time Protocol) с различными профилями
- **NTP** (Network Time Protocol)
- **PPS** (Pulse Per Second) 
- **GNSS/GPS** синхронизация
- **PHC** (Precision Hardware Clock)
- **NMEA** сообщения

### Продвинутые функции
- Первичные и вторичные источники времени
- Настройка интерфейсов и доменов
- Компенсация асимметрии и задержек
- Фильтрация пакетов и обнаружение выбросов

### Мониторинг и управление
- SSH CLI интерфейс
- HTTP веб-интерфейс
- Интеграция с Elasticsearch
- Логирование в syslog
- Dashboards в Kibana

### Конфигурация
- YAML конфигурационный файл
- Поддержка лицензирования
- Гибкие настройки для различных сценариев использования

## Установка

```bash
# Клонируйте репозиторий
git clone <repository-url>
cd shiwatime

# Сборка
go build -o shiwatime cmd/shiwatime/main.go

# Запуск
./shiwatime -config=config/shiwatime.yml
```

## Конфигурация

Основная конфигурация находится в файле `config/shiwatime.yml`. Примеры конфигурации для различных протоколов см. в директории `examples/`.

## Архитектура

```
┌─────────────────────┐
│    CLI Interface    │
├─────────────────────┤
│   HTTP Interface    │
├─────────────────────┤
│  Time Sync Engine   │
├─────────────────────┤
│ Protocol Handlers   │
│ (PTP, NTP, PPS)     │
├─────────────────────┤
│   Clock Manager     │
├─────────────────────┤
│  Metrics & Logging  │
└─────────────────────┘
```

## Использование

### Базовая PTP синхронизация
```yaml
shiwatime:
  clock_sync:
    adjust_clock: true
    primary_clocks:
      - protocol: ptp
        domain: 0
        interface: eth0
```

### NTP синхронизация
```yaml
shiwatime:
  clock_sync:
    adjust_clock: true
    primary_clocks:
      - protocol: ntp
        ip: '0.pool.ntp.org'
        pollinterval: 4s
```

## Лицензия

MIT License