# TimeSource - Универсальный обработчик источников времени

## Обзор

TimeSource - это универсальный обработчик источников времени в ShiwaTime, который предоставляет единый интерфейс для работы с различными протоколами синхронизации времени. Он позволяет конфигурировать и управлять источниками времени через единый API.

## Возможности

- **Универсальный интерфейс**: Единый API для всех типов источников времени
- **Гибкая конфигурация**: Поддержка различных режимов работы и параметров
- **Мониторинг**: Встроенный мониторинг состояния и статистики
- **Веб-интерфейс**: Интеграция с веб-интерфейсом для отображения информации
- **Логирование**: Подробное логирование всех операций

## Поддерживаемые типы источников

TimeSource поддерживает следующие типы источников времени:

- **NTP** - Network Time Protocol
- **PTP** - Precision Time Protocol (IEEE 1588)
- **PPS** - Pulse Per Second
- **PHC** - Precision Hardware Clock
- **NMEA** - GPS/GNSS приемники
- **Timecard** - Специализированные карты точного времени
- **Mock** - Тестовый источник времени

## Конфигурация

### Базовая структура

```yaml
type: "timesource"
timesource_type: "ntp"  # Тип источника времени
timesource_mode: "client"  # Режим работы
timesource_config:  # Специфичная конфигурация
  polling_interval: 64
  max_offset: "1000ms"
  max_delay: "100ms"
  trust: true
```

### Примеры конфигурации

#### NTP источник

```yaml
- type: "timesource"
  host: "192.168.1.100"
  port: 123
  weight: 10
  timesource_type: "ntp"
  timesource_mode: "client"
  timesource_config:
    polling_interval: 64
    max_offset: "1000ms"
    max_delay: "100ms"
    trust: true
```

#### PTP источник

```yaml
- type: "timesource"
  interface: "eth0"
  weight: 8
  timesource_type: "ptp"
  timesource_mode: "slave"
  timesource_config:
    domain: 0
    transport_type: "UDPv4"
    log_announce_interval: 1
    log_sync_interval: 0
    log_delay_req_interval: 0
```

#### PPS источник

```yaml
- type: "timesource"
  device: "/dev/pps0"
  weight: 5
  timesource_type: "pps"
  timesource_mode: "input"
  timesource_config:
    pps_mode: "rising"
    pps_kernel: true
```

#### GPS/NMEA источник

```yaml
- type: "timesource"
  device: "/dev/ttyUSB0"
  weight: 6
  timesource_type: "nmea"
  timesource_mode: "gps"
  timesource_config:
    baud_rate: 9600
    data_bits: 8
    stop_bits: 1
    parity: "none"
```

#### PHC источник

```yaml
- type: "timesource"
  interface: "eth0"
  weight: 4
  timesource_type: "phc"
  timesource_mode: "slave"
  timesource_config:
    phc_index: 0
    hw_timestamping: true
```

## API интерфейс

### TimeSourceHandler

```go
type TimeSourceHandler interface {
    // Start запускает обработчик
    Start() error
    
    // Stop останавливает обработчик
    Stop() error
    
    // GetTimeInfo получает информацию о времени
    GetTimeInfo() (*TimeInfo, error)
    
    // GetStatus получает статус соединения
    GetStatus() ConnectionStatus
    
    // GetConfig получает конфигурацию
    GetConfig() config.TimeSourceConfig
}
```

### Создание обработчика

```go
import "github.com/shiwatime/shiwatime/internal/protocols"

config := config.TimeSourceConfig{
    Type: "timesource",
    TimesourceType: "ntp",
    TimesourceMode: "client",
    Host: "192.168.1.100",
    Port: 123,
    Weight: 10,
}

handler, err := protocols.NewTimeSourceHandler(config, logger)
if err != nil {
    log.Fatal(err)
}

// Запуск обработчика
err = handler.Start()
if err != nil {
    log.Fatal(err)
}

// Получение информации о времени
timeInfo, err := handler.GetTimeInfo()
if err != nil {
    log.Printf("Ошибка получения времени: %v", err)
} else {
    log.Printf("Время: %v, Смещение: %v", timeInfo.Timestamp, timeInfo.Offset)
}

// Получение статуса
status := handler.GetStatus()
log.Printf("Статус: %v", status.Connected)

// Остановка обработчика
handler.Stop()
```

## Веб-интерфейс

TimeSource интегрирован с веб-интерфейсом ShiwaTime. В веб-интерфейсе отображается:

- Тип источника времени
- Режим работы
- Конфигурационные параметры
- Статус соединения
- Статистика работы

### Отображение в веб-интерфейсе

Для TimeSource источников в веб-интерфейсе показывается:

- **Тип источника**: NTP, PTP, PPS, PHC, NMEA, Mock
- **Режим работы**: client, server, slave, master, input, output
- **Конфигурация**: Специфичные параметры для каждого типа

## Мониторинг и логирование

### Логирование

TimeSource генерирует следующие типы логов:

- **INFO**: Запуск/остановка обработчика, успешные операции
- **WARN**: Таймауты, проблемы с соединением
- **ERROR**: Критические ошибки, невозможность запуска

### Метрики

Доступные метрики:

- **ConnectionStatus**: Статус соединения
- **TimeInfo**: Информация о времени и качестве
- **Statistics**: Статистика работы (пакеты, байты, ошибки)

## Устранение неполадок

### Частые проблемы

1. **Ошибка создания обработчика**
   - Проверьте корректность конфигурации
   - Убедитесь, что указанный тип источника поддерживается

2. **Проблемы с соединением**
   - Проверьте доступность хоста/устройства
   - Убедитесь в корректности сетевых настроек

3. **Проблемы с качеством времени**
   - Проверьте настройки качества (max_offset, max_delay)
   - Убедитесь в корректности временных зон

### Отладка

Для включения подробного логирования:

```yaml
logging:
  stdout:
    enable: true
  level: "debug"
```

## Примеры использования

### Полный пример конфигурации

См. файл `config/timesource-example.yml` для полного примера конфигурации с различными типами источников времени.

### Программное использование

```go
package main

import (
    "log"
    "time"
    
    "github.com/shiwatime/shiwatime/internal/config"
    "github.com/shiwatime/shiwatime/internal/protocols"
)

func main() {
    // Конфигурация NTP источника
    config := config.TimeSourceConfig{
        Type: "timesource",
        TimesourceType: "ntp",
        TimesourceMode: "client",
        Host: "pool.ntp.org",
        Port: 123,
        Weight: 10,
        PollingInterval: 64 * time.Second,
        MaxOffset: 1000 * time.Millisecond,
        MaxDelay: 100 * time.Millisecond,
    }
    
    // Создание логгера
    logger := logrus.New()
    logger.SetLevel(logrus.InfoLevel)
    
    // Создание обработчика
    handler, err := protocols.NewTimeSourceHandler(config, logger)
    if err != nil {
        log.Fatal(err)
    }
    
    // Запуск
    if err := handler.Start(); err != nil {
        log.Fatal(err)
    }
    defer handler.Stop()
    
    // Мониторинг
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            timeInfo, err := handler.GetTimeInfo()
            if err != nil {
                log.Printf("Ошибка: %v", err)
                continue
            }
            
            status := handler.GetStatus()
            log.Printf("Время: %v, Смещение: %v, Статус: %v", 
                timeInfo.Timestamp, timeInfo.Offset, status.Connected)
        }
    }
}
```

## Заключение

TimeSource предоставляет универсальный и гибкий способ работы с различными источниками времени в ShiwaTime. Он упрощает конфигурацию и управление источниками времени, предоставляя единый интерфейс для всех поддерживаемых протоколов.