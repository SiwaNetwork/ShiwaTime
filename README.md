# Timebeat Enhanced - Интеграция протоколов и SSH CLI

Этот проект предоставляет расширенные возможности для timebeat с поддержкой всех основных протоколов синхронизации времени и SSH CLI интерфейсом для управления.

## Возможности

### ✅ Интеграция Elastic Beats
- Полная интеграция с Elasticsearch
- Автоматическая отправка метрик
- Мониторинг и алертинг через X-Pack

### ✅ Поддерживаемые протоколы
- **PTP (Precision Time Protocol)** - высокоточная синхронизация времени
- **NTP (Network Time Protocol)** - стандартный сетевой протокол времени
- **PPS (Pulse Per Second)** - синхронизация по импульсам
- **NMEA (GNSS Navigation)** - синхронизация от GPS/GNSS приемников
- **PHC (PTP Hardware Clock)** - аппаратные часы PTP

### ✅ SSH CLI интерфейс
- Удаленное управление через SSH
- Интерактивные команды управления
- Мониторинг в реальном времени
- Включение/отключение протоколов

## Быстрая установка

```bash
# Сделать скрипт исполняемым
chmod +x install_timebeat_enhancements.sh

# Запустить установку от имени root
sudo ./install_timebeat_enhancements.sh
```

## Подробная установка

### 1. Подготовка системы

```bash
# Обновление системы
sudo apt update && sudo apt upgrade -y

# Установка зависимостей
sudo apt install -y python3 python3-pip chrony openssh-server
```

### 2. Установка timebeat

```bash
# Установка deb пакета
sudo dpkg -i timebeat-2.2.20-amd64.deb
sudo apt-get install -f  # Исправление зависимостей если нужно
```

### 3. Настройка конфигурации

```bash
# Копирование конфигурации с активными протоколами
sudo cp timebeat-active-protocols.yml /etc/timebeat/timebeat.yml
sudo cp timebeat.lic /etc/timebeat/

# Настройка прав доступа
sudo chown timebeat:timebeat /etc/timebeat/*
```

### 4. Установка SSH CLI

```bash
# Создание директории
sudo mkdir -p /opt/timebeat-ssh-cli

# Копирование файлов
sudo cp timebeat_ssh_cli.py /opt/timebeat-ssh-cli/
sudo cp requirements.txt /opt/timebeat-ssh-cli/

# Установка Python зависимостей
cd /opt/timebeat-ssh-cli
sudo python3 -m pip install -r requirements.txt

# Генерация SSH ключа
sudo ssh-keygen -t rsa -b 2048 -f /opt/timebeat-ssh-cli/ssh_host_key -N ""
```

### 5. Настройка systemd сервиса

```bash
# Копирование service файла
sudo cp timebeat-ssh-cli.service /etc/systemd/system/

# Перезагрузка systemd и запуск
sudo systemctl daemon-reload
sudo systemctl enable timebeat-ssh-cli
sudo systemctl start timebeat-ssh-cli
```

## Использование SSH CLI

### Подключение

```bash
ssh timebeat@localhost -p 2222
# Пароль: timebeat123
```

### Доступные команды

| Команда | Описание | Пример |
|---------|----------|--------|
| `help` | Показать справку | `help` |
| `status` | Статус сервиса timebeat | `status` |
| `start` | Запустить timebeat | `start` |
| `stop` | Остановить timebeat | `stop` |
| `restart` | Перезапустить timebeat | `restart` |
| `logs [N]` | Показать логи (N строк) | `logs 100` |
| `protocols` | Список протоколов | `protocols` |
| `enable <proto>` | Включить протокол | `enable ptp` |
| `disable <proto>` | Отключить протокол | `disable nmea` |
| `clock` | Статус синхронизации | `clock` |
| `config` | Показать конфигурацию | `config` |
| `reload` | Перезагрузить конфиг | `reload` |
| `exit/quit` | Выйти из сессии | `exit` |

### Пример сессии

```
$ ssh timebeat@localhost -p 2222
Welcome to Timebeat SSH CLI Interface
Type 'help' for available commands

timebeat> protocols
=== PRIMARY CLOCKS ===
  PTP: ENABLED (eth0)
  NTP: ENABLED (pool.ntp.org)
  PPS: ENABLED (eth0)

=== SECONDARY CLOCKS ===
  NMEA: ENABLED (/dev/ttyS0)
  PHC: ENABLED (/dev/ptp0)
  NTP: ENABLED (time.cloudflare.com)

timebeat> status
● timebeat.service - Timebeat
   Loaded: loaded (/lib/systemd/system/timebeat.service; enabled; vendor preset: enabled)
   Active: active (running) since Mon 2024-01-15 10:30:00 UTC; 1h 30min ago
...

timebeat> logs 20
Jan 15 10:30:00 server timebeat[1234]: Starting timebeat...
Jan 15 10:30:01 server timebeat[1234]: PTP protocol initialized on eth0
Jan 15 10:30:02 server timebeat[1234]: NTP client connected to pool.ntp.org
...

timebeat> disable nmea
Protocol NMEA DISABLED in secondary clocks
Timebeat service restarted successfully

timebeat> exit
Goodbye!
```

## Конфигурация протоколов

### PTP (Precision Time Protocol)

```yaml
- protocol: ptp
  domain: 0
  interface: eth0
  profile: 'G.8275.2'
  serve_unicast: true
  serve_multicast: true
```

### NTP (Network Time Protocol)

```yaml
- protocol: ntp
  ip: 'pool.ntp.org'
  pollinterval: 4s
  serve_unicast: true
```

### PPS (Pulse Per Second)

```yaml
- protocol: pps
  interface: eth0
  pin: 0
  edge_mode: "rising"
  linked_device: '/dev/ttyS0'
```

### NMEA (GNSS Navigation)

```yaml
- protocol: nmea
  device: '/dev/ttyS0'
  baud: 9600
  offset: 0
```

### PHC (PTP Hardware Clock)

```yaml
- protocol: phc
  device: '/dev/ptp0'
  offset: 0
```

## Мониторинг и логирование

### Логи системы

```bash
# Логи timebeat
sudo journalctl -u timebeat -f

# Логи SSH CLI
sudo journalctl -u timebeat-ssh-cli -f

# Файловые логи
sudo tail -f /var/log/timebeat/timebeat.log
sudo tail -f /var/log/timebeat/timebeat_ssh_cli.log
```

### Elasticsearch интеграция

Timebeat автоматически отправляет метрики в Elasticsearch:

```yaml
output.elasticsearch:
  hosts: ["localhost:9200"]
  index: "timebeat-%{+yyyy.MM.dd}"
```

### Мониторинг метрик

```bash
# Через SSH CLI
ssh timebeat@localhost -p 2222
timebeat> clock

# Через curl к Elasticsearch
curl -X GET "localhost:9200/timebeat-*/_search?pretty"
```

## Устранение проблем

### Проверка статуса сервисов

```bash
sudo systemctl status timebeat
sudo systemctl status timebeat-ssh-cli
```

### Проверка подключения SSH

```bash
# Проверка порта
ss -tulpn | grep :2222

# Тест подключения
ssh -v timebeat@localhost -p 2222
```

### Проверка конфигурации

```bash
# Валидация YAML конфигурации
python3 -c "import yaml; yaml.safe_load(open('/etc/timebeat/timebeat.yml'))"

# Проверка прав доступа
ls -la /etc/timebeat/
ls -la /var/log/timebeat/
```

### Отладка протоколов

```bash
# Проверка PTP
sudo ptp4l -i eth0 -s

# Проверка NTP
chrony sources -v

# Проверка PPS
sudo cat /sys/class/pps/pps0/assert

# Проверка PHC
sudo phc_ctl /dev/ptp0 get
```

## Файловая структура

```
/etc/timebeat/
├── timebeat.yml          # Основная конфигурация
├── timebeat.lic          # Лицензионный файл
└── peerids.json          # Идентификаторы пиров

/opt/timebeat-ssh-cli/
├── timebeat_ssh_cli.py   # SSH CLI сервер
├── requirements.txt      # Python зависимости
└── ssh_host_key          # SSH ключ хоста

/var/log/timebeat/
├── timebeat.log          # Логи timebeat
└── timebeat_ssh_cli.log  # Логи SSH CLI

/etc/systemd/system/
└── timebeat-ssh-cli.service  # Systemd сервис
```

## Безопасность

### Изменение SSH пароля

Для production среды рекомендуется:

1. Изменить пароль в `timebeat_ssh_cli.py`
2. Использовать SSH ключи вместо паролей
3. Настроить файрвол для ограничения доступа

```python
# В timebeat_ssh_cli.py
await asyncssh.listen(
    host='',
    port=self.port,
    server_host_keys=['ssh_host_key'],
    process_factory=self.handle_client,
    authorized_client_keys='authorized_keys',  # Вместо пароля
)
```

### Настройка файрвола

```bash
# Ограничение доступа только с определенных IP
sudo ufw allow from 192.168.1.0/24 to any port 2222

# Для всех IP (осторожно в production!)
sudo ufw allow 2222/tcp
```

## Поддержка

При возникновении проблем:

1. Проверьте логи: `journalctl -u timebeat-ssh-cli -f`
2. Убедитесь что все сервисы запущены
3. Проверьте конфигурацию на синтаксические ошибки
4. Убедитесь что нужные порты открыты в файрволе