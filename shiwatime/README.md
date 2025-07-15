# ShiwaTime

ShiwaTime - это программа для синхронизации времени, написанная на Go, которая повторяет функциональность Timebeat. Она поддерживает различные протоколы синхронизации времени и интеграцию с Elasticsearch через Beats.

## Возможности

- **Поддержка множества протоколов синхронизации**:
  - NTP (Network Time Protocol)
  - PTP (Precision Time Protocol) - в разработке
  - PPS (Pulse Per Second) - в разработке
  - NMEA (GPS/GNSS) - в разработке
  - PHC (PTP Hardware Clock) - в разработке

- **Гибкая архитектура источников времени**:
  - Первичные источники времени
  - Вторичные источники времени (резервные)
  - Автоматическое переключение между источниками

- **Продвинутые алгоритмы управления часами**:
  - Sigma (по умолчанию)
  - Alpha, Beta, Gamma, Rho (в разработке)
  - Фильтрация выбросов
  - Адаптивная коррекция частоты

- **Мониторинг и метрики**:
  - Интеграция с Elasticsearch
  - HTTP API для мониторинга
  - Подробное логирование

## Установка

### Требования

- Go 1.21 или выше
- Linux с правами root (для синхронизации системных часов)
- Elasticsearch (опционально, для метрик)

### Сборка из исходников

```bash
git clone https://github.com/your-org/shiwatime.git
cd shiwatime
go mod download
go build -o shiwatime cmd/shiwatime/main.go
```

### Установка

```bash
# Создание директорий
sudo mkdir -p /etc/shiwatime
sudo mkdir -p /var/log/shiwatime

# Копирование файлов
sudo cp shiwatime /usr/local/bin/
sudo cp configs/shiwatime.yml /etc/shiwatime/

# Настройка прав
sudo chmod +x /usr/local/bin/shiwatime
```

## Конфигурация

Основной конфигурационный файл находится в `/etc/shiwatime/shiwatime.yml`.

### Пример минимальной конфигурации

```yaml
shiwatime:
  clock_sync:
    adjust_clock: true
    step_limit: 15m
    
    primary_clocks:
      - protocol: ntp
        ip: 'pool.ntp.org'
        pollinterval: 4s
        
    secondary_clocks:
      - protocol: ntp
        ip: 'time.google.com'
        pollinterval: 4s

output:
  elasticsearch:
    hosts: ['localhost:9200']

logging:
  level: info
  to_files: true
  files:
    path: /var/log/shiwatime
```

## Запуск

### Прямой запуск

```bash
sudo shiwatime -c /etc/shiwatime/shiwatime.yml
```

### Параметры командной строки

```
-c string       Путь к конфигурационному файлу (по умолчанию "/etc/shiwatime/shiwatime.yml")
-test          Проверить конфигурацию и выйти
-version       Показать версию
-log-level     Уровень логирования (debug, info, warn, error)
```

### Systemd сервис

Создайте файл `/etc/systemd/system/shiwatime.service`:

```ini
[Unit]
Description=ShiwaTime - Time Synchronization Service
Documentation=https://github.com/your-org/shiwatime
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/shiwatime -c /etc/shiwatime/shiwatime.yml
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Затем:

```bash
sudo systemctl daemon-reload
sudo systemctl enable shiwatime
sudo systemctl start shiwatime
```

## HTTP API

Если HTTP интерфейс включен, доступны следующие эндпоинты:

- `GET /` - Информация о версии
- `GET /status` - Подробный статус сервиса
- `GET /health` - Проверка здоровья
- `GET /metrics` - Метрики (в разработке)

Пример:

```bash
curl http://localhost:8088/status
```

## Разработка

### Структура проекта

```
shiwatime/
├── cmd/shiwatime/      # Основная программа
├── pkg/
│   ├── clock/          # Менеджер часов и интерфейсы
│   ├── config/         # Конфигурация
│   ├── ntp/            # NTP клиент
│   ├── ptp/            # PTP клиент (TODO)
│   ├── steering/       # Алгоритмы управления часами
│   └── metrics/        # Сбор и отправка метрик
├── internal/
│   └── service/        # Основной сервис
├── configs/            # Примеры конфигураций
└── docs/               # Документация
```

### Добавление нового протокола

1. Создайте пакет в `pkg/` для нового протокола
2. Реализуйте интерфейс `clock.TimeSource`
3. Добавьте создание источника в `clock/factory.go`
4. Обновите документацию

### Тестирование

```bash
go test ./...
```

## Roadmap

- [ ] Полная реализация PTP клиента
- [ ] Поддержка PPS
- [ ] Поддержка NMEA/GPS
- [ ] Поддержка PHC
- [ ] Интеграция с Beats для отправки метрик
- [ ] SSH CLI интерфейс
- [ ] Поддержка Windows
- [ ] Веб-интерфейс для мониторинга

## Лицензия

Проект распространяется под лицензией MIT. См. файл LICENSE для деталей.

## Вклад в проект

Мы приветствуем вклад в развитие проекта! Пожалуйста, ознакомьтесь с CONTRIBUTING.md перед отправкой pull request.

## Поддержка

Для вопросов и поддержки:
- Создайте issue на GitHub
- Отправьте email на support@shiwatime.org