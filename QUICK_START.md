# Быстрый запуск Timebeat Enhanced

## 🚀 Быстрая установка (5 минут)

```bash
# 1. Скачать репозиторий и перейти в директорию
cd /workspace

# 2. Запустить автоматическую установку
sudo ./install_timebeat_enhancements.sh
```

## ✅ Что будет установлено

### Протоколы синхронизации времени:
- **PTP** - высокоточная синхронизация (наносекунды)
- **NTP** - стандартный протокол времени 
- **PPS** - импульсная синхронизация
- **NMEA** - GPS/GNSS синхронизация  
- **PHC** - аппаратные часы PTP

### SSH CLI интерфейс:
- Удаленное управление на порту 2222
- Интерактивные команды
- Мониторинг в реальном времени

### Elastic Beats интеграция:
- Автоматическая отправка метрик в Elasticsearch
- Мониторинг через X-Pack

## 🔧 Быстрые команды

### Подключение к SSH CLI:
```bash
ssh timebeat@localhost -p 2222
# Пароль: timebeat123
```

### Основные команды в CLI:
```bash
timebeat> help        # Справка
timebeat> status      # Статус сервиса
timebeat> protocols   # Список протоколов
timebeat> logs 50     # Показать логи
timebeat> enable ptp  # Включить PTP
timebeat> disable ntp # Отключить NTP
```

### Проверка установки:
```bash
# Статус сервисов
sudo systemctl status timebeat
sudo systemctl status timebeat-ssh-cli

# Тест SSH подключения
ssh timebeat@localhost -p 2222
```

## 📁 Важные файлы

| Файл | Назначение |
|------|------------|
| `/etc/timebeat/timebeat.yml` | Основная конфигурация |
| `/var/log/timebeat/` | Логи системы |
| `README.md` | Подробная документация |

## 🛠 Если что-то пошло не так

```bash
# Проверить логи установки
sudo journalctl -u timebeat-ssh-cli -f

# Перезапустить сервисы
sudo systemctl restart timebeat
sudo systemctl restart timebeat-ssh-cli

# Проверить порт SSH CLI
ss -tulpn | grep :2222
```

## 🔗 Что дальше?

1. Подключитесь к SSH CLI: `ssh timebeat@localhost -p 2222`
2. Настройте нужные протоколы командой `enable/disable`
3. Мониторьте метрики в Elasticsearch
4. Изучите полную документацию в `README.md`

---
**Готово!** Теперь у вас есть полнофункциональная система синхронизации времени с удаленным управлением. 🎉