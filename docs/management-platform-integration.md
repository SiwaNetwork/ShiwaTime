# Интеграция с Management Platform

## Обзор

ShiwaTime поддерживает интеграцию с [Timebeat Management Platform](https://github.com/timebeat-app/management-platform) для централизованного мониторинга и управления системами синхронизации времени.

## Архитектура интеграции

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   ShiwaTime     │    │  Management      │    │   Elasticsearch │
│   Instance      │────│  Platform        │────│   Cluster       │
│                 │    │                  │    │                 │
│ • HTTP API      │    │ • Dashboard      │    │ • Metrics       │
│ • SSH CLI       │    │ • Alerts         │    │ • Logs          │
│ • Prometheus    │    │ • Configuration  │    │ • Monitoring    │
│ • Elasticsearch │    │ • Reports        │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

## Компоненты интеграции

### 1. HTTP API (порт 8088)

Предоставляет REST API для мониторинга:

```bash
# Статус системы
curl http://localhost:8088/api/v1/status

# Источники времени
curl http://localhost:8088/api/v1/sources

# Проверка здоровья
curl http://localhost:8088/api/v1/health

# Prometheus метрики
curl http://localhost:8088/metrics
```

### 2. SSH CLI (порт 65129)

Интерактивный CLI для управления:

```bash
# Подключение
ssh -p 65129 admin@localhost

# Основные команды
timebeat> help
timebeat> status
timebeat> protocols
timebeat> logs 50
```

### 3. Elasticsearch интеграция

Отправка метрик и логов в Elasticsearch:

```yaml
output.elasticsearch:
  hosts: ['elastic.customer.timebeat.app:9200']
  protocol: 'https'
  username: 'elastic'
  password: 'changeme'
  ssl.certificate_authorities: ['/etc/timebeat/pki/ca.crt']
  ssl.certificate: '/etc/timebeat/pki/timebeat.crt'
  ssl.key: '/etc/timebeat/pki/timebeat.key'
```

## Быстрая настройка

### 1. Автоматическая настройка

```bash
# Запуск скрипта настройки
sudo ./setup_management_integration.sh

# Тестирование интеграции
./test_management_integration.sh
```

### 2. Ручная настройка

#### Шаг 1: Конфигурация

Создайте файл `/etc/timebeat/timebeat-cloud.yml`:

```yaml
timebeat:
  clock_sync:
    adjust_clock: true
    step_limit: 15m
    
    primary_clocks:
      - protocol: ptp
        domain: 0
        serve_unicast: true
        serve_multicast: true
        server_only: true
        announce_interval: 1
        sync_interval: 0
        delayrequest_interval: 0
        disable: false
        interface: enp0s3

  http:
    enable: true
    bind_port: 8088
    bind_host: 0.0.0.0

  cli:
    enable: true
    bind_port: 65129
    bind_host: 0.0.0.0
    username: "admin"
    password: "changeme"

output.elasticsearch:
  hosts: ['elastic.customer.timebeat.app:9200']
  protocol: 'https'
  username: 'elastic'
  password: 'changeme'
  ssl.certificate_authorities: ['/etc/timebeat/pki/ca.crt']
  ssl.certificate: '/etc/timebeat/pki/timebeat.crt'
  ssl.key: '/etc/timebeat/pki/timebeat.key'

monitoring.enabled: true
monitoring.elasticsearch:
```

#### Шаг 2: SSL сертификаты

```bash
# Создание директории
mkdir -p /etc/timebeat/pki

# CA сертификат
openssl req -x509 -newkey rsa:4096 \
  -keyout /etc/timebeat/pki/ca.key \
  -out /etc/timebeat/pki/ca.crt \
  -days 365 -nodes \
  -subj "/C=US/ST=State/L=City/O=Timebeat/CN=Timebeat CA"

# Клиентский сертификат
openssl req -newkey rsa:4096 \
  -keyout /etc/timebeat/pki/timebeat.key \
  -out /etc/timebeat/pki/timebeat.csr \
  -nodes \
  -subj "/C=US/ST=State/L=City/O=Timebeat/CN=timebeat-client"

# Подпись сертификата
openssl x509 -req \
  -in /etc/timebeat/pki/timebeat.csr \
  -CA /etc/timebeat/pki/ca.crt \
  -CAkey /etc/timebeat/pki/ca.key \
  -CAcreateserial \
  -out /etc/timebeat/pki/timebeat.crt \
  -days 365

# Права доступа
chmod 600 /etc/timebeat/pki/*.key
chmod 644 /etc/timebeat/pki/*.crt
```

#### Шаг 3: Systemd сервис

Создайте `/etc/systemd/system/timebeat-cloud.service`:

```ini
[Unit]
Description=Timebeat Cloud Service
After=network.target
Wants=network.target

[Service]
Type=simple
User=timebeat
Group=timebeat
ExecStart=/usr/bin/timebeat -c /etc/timebeat/timebeat-cloud.yml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

CapabilityBoundingSet=CAP_SYS_TIME CAP_NET_ADMIN CAP_NET_RAW
AmbientCapabilities=CAP_SYS_TIME CAP_NET_ADMIN CAP_NET_RAW

NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/timebeat /etc/timebeat

[Install]
WantedBy=multi-user.target
```

#### Шаг 4: Запуск

```bash
# Создание пользователя
useradd -r -s /bin/false -d /etc/timebeat timebeat

# Перезагрузка systemd
systemctl daemon-reload

# Включение и запуск сервиса
systemctl enable --now timebeat-cloud.service

# Проверка статуса
systemctl status timebeat-cloud.service
```

## Мониторинг и метрики

### Prometheus метрики

Доступны на `http://localhost:8088/metrics`:

```
# HELP shiwatime_clock_state Current clock state
# TYPE shiwatime_clock_state gauge
shiwatime_clock_state 1

# HELP shiwatime_sources_total Total number of time sources
# TYPE shiwatime_sources_total gauge
shiwatime_sources_total 3

# HELP shiwatime_sources_active Number of active time sources
# TYPE shiwatime_sources_active gauge
shiwatime_sources_active 2
```

### Elasticsearch индексы

Автоматически создаются индексы:

- `timebeat-YYYY.MM.DD` - основные метрики
- `shiwatime_clock-YYYY.MM.DD` - метрики часов
- `shiwatime_source-YYYY.MM.DD` - метрики источников

### Kibana дашборды

Импортируйте дашборды из Management Platform:

1. Откройте Kibana
2. Перейдите в Stack Management > Saved Objects
3. Импортируйте дашборды из Management Platform

## Безопасность

### SSL/TLS

Все соединения с Management Platform используют SSL/TLS:

```yaml
ssl.verification_mode: "certificate"
ssl.certificate_authorities: ['/etc/timebeat/pki/ca.crt']
ssl.certificate: '/etc/timebeat/pki/timebeat.crt'
ssl.key: '/etc/timebeat/pki/timebeat.key'
```

### Аутентификация

```yaml
# API Key аутентификация
api_key: 'id:api_key'

# Или username/password
username: 'elastic'
password: 'changeme'
```

### Firewall

```bash
# UFW
ufw allow 8088/tcp comment "Timebeat HTTP API"
ufw allow 65129/tcp comment "Timebeat SSH CLI"

# iptables
iptables -A INPUT -p tcp --dport 8088 -j ACCEPT
iptables -A INPUT -p tcp --dport 65129 -j ACCEPT
```

## Устранение неполадок

### Проверка подключения

```bash
# Тест HTTP API
curl -v http://localhost:8088/api/v1/status

# Тест SSH CLI
ssh -v -p 65129 admin@localhost

# Тест Elasticsearch
curl -v -u elastic:changeme \
  https://elastic.customer.timebeat.app:9200/_cluster/health
```

### Проверка логов

```bash
# Systemd логи
journalctl -u timebeat-cloud.service -f

# Файловые логи
tail -f /var/log/timebeat/timebeat

# Проверка ошибок
grep -i error /var/log/timebeat/timebeat
```

### Диагностика SSL

```bash
# Проверка сертификатов
openssl x509 -in /etc/timebeat/pki/timebeat.crt -text -noout

# Тест SSL соединения
openssl s_client -connect elastic.customer.timebeat.app:9200 \
  -cert /etc/timebeat/pki/timebeat.crt \
  -key /etc/timebeat/pki/timebeat.key \
  -CAfile /etc/timebeat/pki/ca.crt
```

## Конфигурация Management Platform

### Настройка аутентификации

В Management Platform настройте:

1. **API Keys** для аутентификации клиентов
2. **SSL сертификаты** для безопасного соединения
3. **Пользователей** с соответствующими правами

### Настройка алертов

Создайте алерты для:

- Потеря синхронизации времени
- Недоступность источников времени
- Высокий offset от эталонного времени
- Ошибки в логах

### Настройка дашбордов

Импортируйте готовые дашборды или создайте собственные для:

- Общего состояния системы
- Детальной информации об источниках
- Исторических данных синхронизации
- Производительности системы

## Обновления и миграция

### Обновление конфигурации

```bash
# Создание резервной копии
cp /etc/timebeat/timebeat-cloud.yml /etc/timebeat/timebeat-cloud.yml.backup

# Обновление конфигурации
sudo ./setup_management_integration.sh

# Перезапуск сервиса
systemctl restart timebeat-cloud.service
```

### Миграция данных

```bash
# Экспорт конфигурации
timebeat export config > config-backup.json

# Импорт конфигурации
timebeat import config < config-backup.json
```

## Поддержка

Для получения поддержки:

1. **Документация**: [Management Platform Docs](https://github.com/timebeat-app/management-platform)
2. **Issues**: [GitHub Issues](https://github.com/timebeat-app/management-platform/issues)
3. **Discord**: [Timebeat Community](https://discord.gg/timebeat)

## Лицензирование

Интеграция с Management Platform требует действующей лицензии Timebeat. Получите лицензию на [timebeat.app](https://timebeat.app).