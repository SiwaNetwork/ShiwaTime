# PTP+Squared - Распределенная P2P синхронизация времени

## Обзор

PTP+Squared - это инновационная реализация распределенной синхронизации времени на базе протокола libp2p. Система позволяет создавать децентрализованную сеть узлов, которые автоматически обнаруживают друг друга и обмениваются информацией о времени для достижения высокой точности синхронизации.

## Архитектура

### Основные компоненты

1. **libp2p Host** - P2P узел для сетевого взаимодействия
2. **PubSub** - система обмена сообщениями между узлами
3. **mDNS Discovery** - автоматическое обнаружение соседних узлов
4. **Seat Management** - управление слот-системой для контроля нагрузки
5. **Time Sync Engine** - алгоритмы синхронизации времени

### Сетевая топология

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Node A    │────│   Node B    │────│   Node C    │
│ (Master)    │    │ (Slave)     │    │ (Slave)     │
└─────────────┘    └─────────────┘    └─────────────┘
       │                   │                   │
       └───────────────────┼───────────────────┘
                           │
                    ┌─────────────┐
                    │   Node D    │
                    │ (Observer)  │
                    └─────────────┘
```

## Протокол сообщений

### Типы сообщений

1. **Time Sync** - синхронизация времени между узлами
2. **Seat Request** - запрос слота у другого узла
3. **Seat Offer** - предложение слота
4. **Seat Accept** - принятие предложения слота
5. **Seat Reject** - отказ от предложения слота
6. **Heartbeat** - проверка активности узла

### Формат сообщения

```json
{
  "type": "time_sync",
  "peer_id": "QmPeerID...",
  "timestamp": "2024-01-15T10:30:00Z",
  "domain": 115,
  "data": {
    "time_info": {
      "offset": 1250000,
      "delay": 50000,
      "quality": 255
    }
  }
}
```

## Конфигурация

### Основные параметры

```yaml
ptpsquared:
  enable: true
  
  # Настройки обнаружения
  discovery:
    mdns: true
    dht: false
    dht_seed_list: 
      - "/ip4/10.101.101.23/tcp/65107/p2p/16Uiu2HAmJiQvJQbja8pf5dKAZsSYxWmcDCxZaoYbMUL5X7GnXej9"
  
  # Сетевые настройки
  domains: [115, 116]
  interface: eth0
  
  # Управление емкостью
  seats_to_offer: 4
  seats_to_fill: 3
  concurrent_sources: 1
  
  # Качество и предпочтения
  capabilities: ["hqosc-1500"]
  preference_score: 0
  reservations: ["1500:50%:115,116", "750:25%"]
```

### Параметры источника времени

```yaml
primary_clocks:
  - type: ptpsquared
    interface: eth0
    domains: [115, 116]
    seats_to_offer: 4
    seats_to_fill: 3
    concurrent_sources: 1
    capabilities: ["hqosc-1500"]
    preference_score: 0
    reservations: ["1500:50%:115,116", "750:25%"]
    group: ptpsquared_primary
    logsource: 'PTP+Squared Primary'
```

## Алгоритмы синхронизации

### 1. Обнаружение узлов

- **mDNS** - локальное обнаружение в той же сети
- **DHT** - глобальное обнаружение через распределенную хеш-таблицу
- **Seed Nodes** - подключение к известным узлам

### 2. Управление слотами

Система использует концепцию "слотов" для контроля нагрузки:

- **Seats to Offer** - количество слотов, которые узел может предложить
- **Seats to Fill** - количество слотов, которые узел хочет заполнить
- **Concurrent Sources** - количество одновременных источников времени

### 3. Выбор лучшего источника

Алгоритм выбора основан на:
- Качестве источника (offset, delay, jitter)
- Предпочтительном балле узла
- Расстоянии в сети (количество хопов)
- Доступности слотов

### 4. Синхронизация времени

```go
func (h *ptpsquaredHandler) calculateQuality(timeInfo *TimeInfo) float64 {
    offsetQuality := 1.0 - float64(timeInfo.Offset.Abs())/float64(time.Second)
    delayQuality := 1.0 - float64(timeInfo.Delay)/float64(time.Second)
    
    if offsetQuality < 0 {
        offsetQuality = 0
    }
    if delayQuality < 0 {
        delayQuality = 0
    }
    
    return (offsetQuality + delayQuality) / 2.0
}
```

## Мониторинг и метрики

### Статистика сети

```go
type PTPSquaredNetworkStats struct {
    TotalPeers       int
    ActivePeers      int
    TotalSeatsOffered int
    TotalSeatsFilled  int
    AverageLatency    time.Duration
    AverageJitter     time.Duration
    NetworkQuality    float64
}
```

### API эндпоинты

- `GET /api/v1/ptpsquared/peers` - список подключенных пиров
- `GET /api/v1/ptpsquared/stats` - статистика сети
- `GET /api/v1/ptpsquared/seats` - информация о слотах
- `POST /api/v1/ptpsquared/request-seat` - запрос слота
- `POST /api/v1/ptpsquared/offer-seat` - предложение слота

## Безопасность

### Криптографические примитивы

- **Ed25519** - для генерации ключей узлов
- **Noise Protocol** - для защищенного обмена сообщениями
- **libp2p Security** - встроенные механизмы безопасности

### Управление ключами

```yaml
ptpsquared:
  keypath: "/etc/timebeat/ptp2key.private"
```

## Производительность

### Ожидаемые характеристики

- **Точность**: ±1-10 микросекунд (в зависимости от сети)
- **Задержка**: 1-10 миллисекунд (локальная сеть)
- **Масштабируемость**: до 1000+ узлов в сети
- **Отказоустойчивость**: автоматическое переключение на резервные источники

### Оптимизации

1. **Кэширование** - кэширование информации о пирах
2. **Батчинг** - группировка сообщений для снижения нагрузки
3. **Фильтрация** - отсев некачественных источников
4. **Адаптивные интервалы** - динамическая настройка частоты синхронизации

## Использование

### Запуск с PTP+Squared

```bash
# Запуск с конфигурацией PTP+Squared
./shiwatime -c config/ptpsquared-example.yml

# Проверка статуса
curl http://localhost:8088/api/v1/ptpsquared/stats

# Просмотр подключенных пиров
curl http://localhost:8088/api/v1/ptpsquared/peers
```

### CLI команды

```bash
# Подключение к CLI
ssh admin@localhost -p 65129

# Команды PTP+Squared
timebeat> ptpsquared status
timebeat> ptpsquared peers
timebeat> ptpsquared stats
timebeat> ptpsquared request-seat <peer_id> <domain>
timebeat> ptpsquared offer-seat <peer_id> <domain>
```

## Отладка

### Логирование

```yaml
logging:
  level: debug
  ptpsquared:
    debug: true
```

### Метрики Prometheus

```
# Метрики PTP+Squared
ptpsquared_peers_total
ptpsquared_peers_active
ptpsquared_seats_offered
ptpsquared_seats_filled
ptpsquared_latency_seconds
ptpsquared_jitter_seconds
ptpsquared_quality_ratio
```

## Совместимость

### Поддерживаемые профили PTP

- **ITU-T G.8265.1** - Telecom Profile
- **ITU-T G.8275.1** - Telecom Profile with full timing support
- **ITU-T G.8275.2** - Telecom Profile with partial timing support
- **IEEE C37.238** - Power Profile
- **IEEE 802.1AS** - Audio/Video Profile
- **IEC/IEEE 61850-9-3** - Power Profile

### Интеграция с существующими системами

PTP+Squared может работать параллельно с:
- Стандартными PTP мастерами
- NTP серверами
- GNSS приемниками
- Аппаратными Timecard

## Заключение

PTP+Squared представляет собой современный подход к распределенной синхронизации времени, сочетающий преимущества P2P архитектуры с высокой точностью PTP протокола. Система обеспечивает отказоустойчивость, масштабируемость и автоматическое управление ресурсами.