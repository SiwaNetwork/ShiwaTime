# Timebeat Unified Complete Configuration

Этот документ описывает единую конфигурацию Timebeat, которая объединяет все настройки в один файл.

## Обзор

Единая конфигурация объединяет все протоколы и функции Timebeat в один комплексный конфигурационный файл. Это включает поддержку **Timebeat Timecard Mini** и все активные протоколы синхронизации времени.

## Структура файлов

### 1. `timebeat-unified-complete.yml` - Единый конфигурационный файл
Это полный единый конфигурационный файл, который включает:
- **PTP (Precision Time Protocol)** конфигурация - АКТИВНА
- **NTP (Network Time Protocol)** конфигурация - АКТИВНА
- **PPS (Pulse Per Second)** конфигурация - АКТИВНА
- **NMEA-GNSS** конфигурация - АКТИВНА
- **PHC (PTP Hardware Clock)** конфигурация - АКТИВНА
- **Timebeat Timecard Mini** конфигурация - АКТИВНА
- **Elasticsearch** настройки вывода
- **Логирование и мониторинг** настройки
- **Все расширенные настройки** и примеры

### 2. `fields.yml` - Отдельный файл полей Elasticsearch
- Остается как отдельный файл (как требовалось)
- Определяет структуру данных для Elasticsearch
- Содержит полную схему ECS (Elastic Common Schema)

### 3. `replace_config.sh` - Скрипт замены конфигурации
- Автоматически заменяет старый конфигурационный файл на новый
- Создает резервную копию
- Проверяет синтаксис
- Перезапускает службу

## Преимущества единой конфигурации

1. **Упрощение управления**: Один файл вместо нескольких
2. **Полная функциональность**: Все протоколы и настройки в одном месте
3. **Активные настройки**: Все протоколы готовы к использованию
4. **Документация**: Подробные комментарии и примеры
5. **Безопасность**: Автоматическое создание резервных копий

## Установка и настройка

### Использование скрипта замены

```bash
# Запуск скрипта замены (требует sudo)
sudo ./replace_config.sh
```

Скрипт выполнит следующие действия:
1. Создаст резервную копию текущего конфигурационного файла
2. Скопирует новый единый конфигурационный файл
3. Установит правильные права доступа
4. Проверит синтаксис конфигурации
5. Перезапустит службу Timebeat
6. Проверит статус службы

### Ручная установка

1. **Создание резервной копии**:
   ```bash
   sudo cp /etc/timebeat/timebeat.yml /etc/timebeat/timebeat.yml.backup
   ```

2. **Установка единой конфигурации**:
   ```bash
   sudo cp timebeat-unified-complete.yml /etc/timebeat/timebeat.yml
   sudo chmod 644 /etc/timebeat/timebeat.yml
   sudo chown root:root /etc/timebeat/timebeat.yml
   ```

3. **Проверка конфигурации**:
   ```bash
   sudo timebeat test config -c /etc/timebeat/timebeat.yml
   ```

4. **Перезапуск службы**:
   ```bash
   sudo systemctl restart timebeat
   ```

## Активные протоколы в единой конфигурации

### Первичные часы (Primary Clocks)
- **PTP Domain 0**: Активен с поддержкой unicast/multicast
- **NTP**: pool.ntp.org как основной источник
- **PPS**: Аппаратный PPS сигнал

### Вторичные часы (Secondary Clocks)
- **NMEA-GNSS**: Последовательный порт /dev/ttyS0
- **PHC**: Аппаратные часы /dev/ptp0
- **Timebeat Timecard Mini**: /dev/ttyS4 с GPS/GLONASS/Galileo
- **Backup NTP**: time.cloudflare.com как резервный

### PHC синхронизация
- Включена синхронизация PHC
- Настроены стратегии смещения и сглаживания
- Включены фильтры выбросов

## Группировка источников времени

Конфигурация организует источники времени в группы:

- **Первичные**: `ptp_primary`, `ntp_primary`, `pps_primary`
- **Вторичные**: `nmea_secondary`, `phc_secondary`, `timecard_mini_secondary`, `ntp_backup`

Это позволяет лучше управлять и мониторить различные источники времени.

## Мониторинг

### Elasticsearch интеграция
Конфигурация включает настройки вывода в Elasticsearch:

```yaml
output.elasticsearch:
  hosts: ["localhost:9200"]
  index: "timebeat-%{+yyyy.MM.dd}"
```

### Логирование
Настроено комплексное логирование:
```yaml
logging.to_files: true
logging.files:
  path: /var/log/timebeat
  name: timebeat
  rotateeverybytes: 10485760
  keepfiles: 7
```

## Проверка работы

### Проверка статуса службы
```bash
systemctl status timebeat
```

### Просмотр логов
```bash
journalctl -u timebeat -f
```

### Проверка конфигурации
```bash
timebeat test config -c /etc/timebeat/timebeat.yml
```

## Устранение неполадок

### Частые проблемы

1. **Служба не запускается**:
   ```bash
   journalctl -u timebeat -n 50
   timebeat test config
   ```

2. **Ошибки синтаксиса**:
   - Проверьте отступы в YAML
   - Убедитесь в правильности структуры

3. **Проблемы с правами доступа**:
   ```bash
   sudo chmod 644 /etc/timebeat/timebeat.yml
   sudo chown root:root /etc/timebeat/timebeat.yml
   ```

### Восстановление из резервной копии
```bash
sudo cp /etc/timebeat/timebeat.yml.backup /etc/timebeat/timebeat.yml
sudo systemctl restart timebeat
```

## Отличия от предыдущих версий

1. **Единый файл**: Все настройки в одном файле вместо нескольких
2. **Активные протоколы**: Все протоколы включены и готовы к работе
3. **Улучшенная документация**: Подробные комментарии для каждого раздела
4. **Автоматизация**: Скрипт для безопасной замены конфигурации
5. **Сохранение fields.yml**: Остается как отдельный файл

## Заключение

Единая конфигурация Timebeat предоставляет полную функциональность в одном файле, упрощая управление и развертывание системы синхронизации времени. Все протоколы активны и готовы к использованию, а файл `fields.yml` остается отдельным для совместимости с Elasticsearch.