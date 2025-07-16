# OCP Timecard Support

## Обзор

OCP Timecard (Open Compute Project Time Appliance Project Time Card) - это специализированная карта точного времени, разработанная для высокоточных приложений синхронизации времени. Данная реализация обеспечивает полную поддержку OCP Timecard в системе Timebeat.

## Возможности

- **Поддержка OCP Timecard**: Полная интеграция с картами OCP Time Appliance Project
- **GNSS/GPS синхронизация**: Поддержка GPS, Galileo, SBAS, BeiDou, QZSS, GLONASS
- **Атомные часы**: Поддержка различных типов осцилляторов (Timebeat RB-QL, SA45, Fusion-XT, OCXO-ROD)
- **SMA порты**: Конфигурируемые SMA порты для входа/выхода сигналов
- **PTP поддержка**: Интеграция с Precision Time Protocol
- **PHC поддержка**: Работа с Precision Hardware Clock
- **SHM поддержка**: Shared Memory для chrony/ntpd
- **PCI драйвер**: Прямой доступ к регистрам через PCI BAR0

## Конфигурация

### Основные параметры

```yaml
- protocol:          ocp_timecard        # Протокол OCP Timecard
  ocp_device:        0                   # ID устройства (/sys/class/timecard/ocpX)
  oscillator_type:   'timebeat-rb-ql'    # Тип осциллятора
  card_config:                           # Конфигурация карты
    - 'sma1:out:mac'                     # SMA1 как выход MAC
    - 'sma2:in:gnss1'                    # SMA2 как вход GNSS1
    - 'gnss1:signal:gps+galileo+sbas'    # GNSS1 сигналы
    - 'osc:type:timebeat-rb-ql'          # Тип осциллятора
  offset:            0                   # Статический offset в наносекундах
  atomic:            false               # Атомный осциллятор
  monitor_only:      false               # Только мониторинг
  weight:            10                  # Приоритет источника
```

### Поддерживаемые типы осцилляторов

- `timebeat-rb-ql` - Timebeat Rubidium Quartz Locked
- `timebeat-rb-sa45` - Timebeat Rubidium SA45
- `timebeat-fusion-xt` - Timebeat Fusion XT
- `timebeat-ocxo-rod` - Timebeat OCXO Rubidium

### Конфигурация SMA портов

SMA порты можно настроить для различных функций:

```yaml
card_config:
  - 'sma1:out:mac'      # SMA1 как выход MAC (атомные часы)
  - 'sma2:in:gnss1'     # SMA2 как вход GNSS1
  - 'sma3:out:phc'      # SMA3 как выход PHC
  - 'sma4:in:gnss2'     # SMA4 как вход GNSS2
```

### Конфигурация GNSS

Поддерживаемые GNSS сигналы:

```yaml
card_config:
  - 'gnss1:signal:gps+galileo+sbas'     # GPS + Galileo + SBAS
  - 'gnss2:signal:gps+galileo'          # GPS + Galileo
```

Доступные сигналы:
- `gps` - GPS L1/L2/L5
- `galileo` - Galileo E1/E5a/E5b
- `sbas` - SBAS
- `beidou` - BeiDou B1/B2/B3
- `qzss` - QZSS L1/L2/L5
- `glonass` - GLONASS L1/L2

## Установка и настройка

### 1. Установка драйвера

```bash
# Клонирование репозитория драйвера
git clone https://github.com/Time-Appliances-Project/Time-Card.git
cd Time-Card/DRV

# Компиляция и установка драйвера
make
sudo modprobe ptp_ocp
```

### 2. Проверка устройства

После установки драйвера проверьте наличие устройства:

```bash
ls -la /sys/class/timecard/ocp0/
```

Должны появиться файлы:
- `available_clock_sources`
- `available_sma_inputs`
- `available_sma_outputs`
- `clock_source`
- `gnss_sync`
- `sma1_in`, `sma2_in`, `sma3_out`, `sma4_out`
- `ttyGNSS`, `ttyMAC`, `ttyNMEA`

### 3. Настройка конфигурации

Создайте конфигурационный файл на основе примера:

```bash
cp config/ocp_timecard_example.yml /etc/timebeat/timebeat.yml
```

Отредактируйте параметры под вашу систему.

### 4. Запуск Timebeat

```bash
sudo timebeat -c /etc/timebeat/timebeat.yml
```

## Мониторинг и диагностика

### Проверка статуса

```bash
# Проверка статуса устройства
cat /sys/class/timecard/ocp0/gnss_sync

# Проверка PTP часов
phc_ctl /dev/ptp4 get

# Проверка GNSS
cat /sys/class/timecard/ocp0/ttyGNSS
```

### Логирование

Timebeat выводит подробные логи о работе OCP Timecard:

```
INFO Starting OCP Timecard handler device=0 path=/sys/class/timecard/ocp0
INFO Configuring GNSS signals gnss=gnss1 signals=gps+galileo+sbas
INFO Configuring oscillator type type=timebeat-rb-ql
INFO ocp-timecard: PCI BAR0 mapped pci_addr=0000:02:00.0
```

### Метрики

Система собирает следующие метрики:
- PPS счетчик и время
- GNSS статус и позиция
- Качество сигнала
- Offset и jitter
- Статус осциллятора

## Устранение неполадок

### Устройство не найдено

```bash
# Проверьте наличие драйвера
lsmod | grep ptp_ocp

# Проверьте PCI устройство
lspci | grep -i timecard

# Перезагрузите драйвер
sudo modprobe -r ptp_ocp
sudo modprobe ptp_ocp
```

### GNSS не работает

```bash
# Проверьте антенну
cat /sys/class/timecard/ocp0/gnss_sync

# Проверьте сигналы
cat /sys/class/timecard/ocp0/available_sma_inputs
```

### PTP не синхронизируется

```bash
# Проверьте PTP статус
phc_ctl /dev/ptp4 get

# Проверьте сетевые интерфейсы
ip link show
```

## Производительность

OCP Timecard обеспечивает:
- Точность синхронизации: < 100 нс
- Стабильность частоты: < 1e-11
- Поддержка leap seconds
- Автоматическое переключение на holdover

## Совместимость

- **Ядро Linux**: 5.12+
- **Архитектуры**: x86_64, ARM64
- **Дистрибутивы**: Ubuntu 20.04+, CentOS 8+, RHEL 8+
- **Драйвер**: ptp_ocp (включен в mainline kernel с 5.15)

## Лицензия

OCP Timecard драйвер распространяется под лицензией GPL-2.0.

## Поддержка

Для получения поддержки:
- [OCP Time Appliance Project](https://github.com/Time-Appliances-Project/Time-Card)
- [Timebeat Documentation](https://docs.timebeat.com)
- [Community Forum](https://community.timebeat.com)