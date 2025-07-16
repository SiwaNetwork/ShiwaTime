# ShiwaTime - Продвинутая система синхронизации точного времени

ShiwaTime - это высокопроизводительная система синхронизации времени, написанная на Go, которая реплицирует и расширяет функциональность Timebeat с поддержкой продвинутых протоколов синхронизации времени.

## ✨ Новые продвинутые функции

### 🎯 Реализованные протоколы

- **✅ NTP (Network Time Protocol)** - Полная реализация клиента с расчетом offset/delay
- **✅ PTP (Precision Time Protocol IEEE 1588)** - Высокоточная синхронизация с аппаратными метками времени:
  - Поддержка multicast и unicast режимов
  - Аппаратные метки времени (Hardware Timestamping)
  - Автоматическое обнаружение PHC устройств
  - Полный стек сообщений: Sync, Follow_Up, Delay_Req, Delay_Resp, Announce
  - Поддержка различных доменов PTP
- **✅ PPS (Pulse Per Second)** - Микросекундная точность от аппаратных сигналов:
  - Поддержка /dev/pps* устройств
  - GPIO интерфейс для Raspberry Pi и подобных
  - Детектирование фронтов: rising, falling, both
  - Аппаратная и программная обработка событий
- **✅ PHC (Precision Hardware Clock)** - Работа с аппаратными часами сетевых карт:
  - Прямое взаимодействие с PHC через ioctl
  - Измерение offset между PHC и системными часами
  - Корректировка частоты PHC
  - Поддержка внешних меток времени
- **🚧 NMEA** - Синхронизация с GPS/GNSS приемников (базовая реализация)
- **🚧 Timecard** - Поддержка специализированных карт точного времени (заготовка)

### 🧠 Продвинутая логика управления часами

- **PID контроллер** для плавной подстройки частоты системных часов
- **Allan Deviation** для анализа стабильности часов
- **Корреляционный анализ** для оценки качества источников
- **Sigma/Rho пороги** для определения стабильности синхронизации
- **Адаптивная фильтрация** с настраиваемым окном усреднения
- **Интеллектуальный выбор источников** с весовыми коэффициентами

### 🔧 Аппаратная интеграция

- **Kernel-level синхронизация** через adjtimex syscalls
- **Аппаратные метки времени** для минимизации джиттера
- **GPIO поддержка** для PPS сигналов
- **PHC интеграция** с автоматическим обнаружением
- **Cross-timestamping** для синхронизации PHC и системных часов

### 📊 Расширенный мониторинг

- **Веб-интерфейс в реальном времени** с современным дизайном:
  - Протокол-специфичная информация (PTP домены, PPS события, PHC статистика)
  - Визуализация стабильности часов
  - Allan Deviation и корреляционные метрики
  - Автообновление каждые 10 секунд
- **REST API** с полной статистикой:
  - `/api/v1/status` - общее состояние системы
  - `/api/v1/sources` - детальная информация об источниках
  - `/api/v1/statistics` - расширенная статистика синхронизации
  - `/metrics` - Prometheus метрики
- **SSH CLI интерфейс** для удаленного управления
- **Elasticsearch интеграция** для long-term мониторинга

## 🚀 Установка и запуск

### Сборка из исходников

```bash
git clone https://github.com/shiwatime/shiwatime
cd shiwatime
make build
```

### Docker контейнер

```bash
docker build -t shiwatime .
docker run -d --name shiwatime \
  --privileged \
  --network host \
  -v ./config:/etc/shiwatime \
  shiwatime
```

### Конфигурация

Создайте файл `config/shiwatime.yml`:

```yaml
shiwatime:
  clock_sync:
    adjust_clock: true
    step_limit: "1s"
    primary_clocks:
      # PTP источник с аппаратными метками времени
      - type: ptp
        interface: enp1s0
        domain: 0
        transport_type: UDPv4
        weight: 10
        
      # NTP fallback
      - type: ntp
        host: pool.ntp.org
        port: 123
        weight: 5
        
      # PPS для высокой точности
      - type: pps
        device: /dev/pps0
        pps_mode: rising
        weight: 15
        
      # PHC для наносекундной точности
      - type: phc
        interface: enp1s0
        phc_index: 0
        weight: 20

  http:
    enabled: true
    bind_address: "0.0.0.0"
    bind_port: 8088

  cli:
    enabled: true
    bind_address: "0.0.0.0"
    bind_port: 65129

  logging:
    level: info
    format: json
```

## 📈 Использование

### Веб-интерфейс

Откройте браузер: `http://localhost:8088`

Веб-интерфейс показывает:
- Текущее состояние системных часов
- Активные источники времени с протокол-специфичной информацией
- Расширенную статистику (Allan Deviation, корреляция, частотное смещение)
- Стабильность синхронизации в реальном времени

### CLI интерфейс

```bash
# Подключение через SSH
ssh localhost -p 65129

# Доступные команды:
> status    # Общее состояние
> sources   # Информация об источниках
> help      # Справка
> exit      # Выход
```

### REST API

```bash
# Общий статус
curl http://localhost:8088/api/v1/status

# Источники времени
curl http://localhost:8088/api/v1/sources

# Расширенная статистика
curl http://localhost:8088/api/v1/statistics

# Prometheus метрики
curl http://localhost:8088/metrics
```

## 🔬 Продвинутые функции

### PTP с аппаратными метками времени

```yaml
- type: ptp
  interface: enp1s0
  domain: 0
  transport_type: UDPv4
  log_announce_interval: 1
  log_sync_interval: 0
  clock_class: 248
  priority1: 128
  priority2: 128
```

### PPS с GPIO поддержкой

```yaml
- type: pps
  gpio_pin: 18  # Raspberry Pi GPIO pin
  pps_mode: rising
  pps_kernel: true
```

### PHC интеграция

```yaml
- type: phc
  interface: enp1s0
  phc_index: 0  # Автоматическое определение
```

### Расширенная логика управления

Система автоматически вычисляет:
- **Allan Deviation** для оценки стабильности
- **Корреляционные коэффициенты** между измерениями
- **PID коррекцию** для плавной подстройки
- **Автоматический выбор** лучшего источника

## 📊 Метрики и статистика

ShiwaTime предоставляет детальную статистику:

- **Временные метрики**: offset, delay, jitter
- **Качественные показатели**: Allan deviation, корреляция
- **Аппаратные метрики**: PHC offset, частотная коррекция
- **Статистика протоколов**: PTP порт состояния, PPS события
- **Системная информация**: kernel sync статус, стабильность

## 🛠 Разработка

### Архитектура

```
internal/
├── clock/          # Менеджер часов с PID контроллером
├── config/         # Конфигурация с полной Timebeat совместимостью
├── protocols/      # Протокольные обработчики
│   ├── ntp.go     # NTP клиент
│   ├── ptp.go     # PTP клиент с HW timestamping
│   ├── pps.go     # PPS обработчик
│   ├── phc.go     # PHC интерфейс
│   ├── nmea.go    # NMEA парсер
│   └── timecard.go # Timecard драйвер
├── server/         # HTTP/CLI серверы
└── metrics/        # Elasticsearch интеграция
```

### Добавление новых протоколов

1. Реализуйте интерфейс `TimeSourceHandler`
2. Добавьте в `protocols/factory.go`
3. Обновите конфигурацию в `config/config.go`
4. Добавьте специфичную логику в веб-интерфейс

## 📝 Лицензия

MIT License - см. файл [LICENSE](LICENSE)

## 🤝 Участие в разработке

Приветствуются pull requests! Особенно интересны:

- Поддержка дополнительных Timecard устройств
- Улучшения PTP стека
- Графические интерфейсы для мониторинга
- Интеграция с системами мониторинга

## 🔗 Связанные проекты

- [Timebeat](https://timebeat.app) - Оригинальная реализация
- [PTP4L](http://linuxptp.sourceforge.net/) - Linux PTP стек
- [chrony](https://chrony.tuxfamily.org/) - NTP реализация