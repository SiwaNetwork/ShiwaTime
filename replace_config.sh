#!/bin/bash

# Скрипт для замены конфигурационного файла Timebeat на единый файл
# Заменяет timebeat.yml на timebeat-unified-complete.yml

set -e

echo "=== Замена конфигурационного файла Timebeat ==="

# Проверяем, что мы запущены с правами root
if [ "$EUID" -ne 0 ]; then
    echo "Ошибка: Этот скрипт должен быть запущен с правами root (sudo)"
    exit 1
fi

# Проверяем существование нового конфигурационного файла
if [ ! -f "timebeat-unified-complete.yml" ]; then
    echo "Ошибка: Файл timebeat-unified-complete.yml не найден в текущей директории"
    exit 1
fi

# Создаем резервную копию текущего конфигурационного файла
if [ -f "/etc/timebeat/timebeat.yml" ]; then
    echo "Создание резервной копии текущего конфигурационного файла..."
    cp /etc/timebeat/timebeat.yml /etc/timebeat/timebeat.yml.backup.$(date +%Y%m%d_%H%M%S)
    echo "Резервная копия создана: /etc/timebeat/timebeat.yml.backup.$(date +%Y%m%d_%H%M%S)"
else
    echo "Предупреждение: Текущий конфигурационный файл /etc/timebeat/timebeat.yml не найден"
fi

# Копируем новый конфигурационный файл
echo "Копирование нового единого конфигурационного файла..."
cp timebeat-unified-complete.yml /etc/timebeat/timebeat.yml

# Устанавливаем правильные права доступа
echo "Установка прав доступа..."
chmod 644 /etc/timebeat/timebeat.yml
chown root:root /etc/timebeat/timebeat.yml

# Проверяем синтаксис конфигурации
echo "Проверка синтаксиса конфигурации..."
if command -v timebeat &> /dev/null; then
    if timebeat test config -c /etc/timebeat/timebeat.yml; then
        echo "✓ Конфигурация прошла проверку синтаксиса"
    else
        echo "✗ Ошибка в синтаксисе конфигурации"
        echo "Восстанавливаем резервную копию..."
        if [ -f "/etc/timebeat/timebeat.yml.backup.$(date +%Y%m%d_%H%M%S)" ]; then
            cp /etc/timebeat/timebeat.yml.backup.$(date +%Y%m%d_%H%M%S) /etc/timebeat/timebeat.yml
        fi
        exit 1
    fi
else
    echo "Предупреждение: Команда timebeat не найдена, пропускаем проверку синтаксиса"
fi

# Перезапускаем службу Timebeat
echo "Перезапуск службы Timebeat..."
if systemctl is-active --quiet timebeat; then
    systemctl restart timebeat
    echo "✓ Служба Timebeat перезапущена"
else
    echo "Предупреждение: Служба Timebeat не запущена"
fi

# Проверяем статус службы
echo "Проверка статуса службы..."
if systemctl is-active --quiet timebeat; then
    echo "✓ Служба Timebeat работает"
else
    echo "✗ Служба Timebeat не работает"
    echo "Проверьте логи: journalctl -u timebeat -n 50"
fi

echo ""
echo "=== Замена завершена ==="
echo ""
echo "Новый единый конфигурационный файл включает:"
echo "- Все активные протоколы синхронизации времени"
echo "- Поддержку Timebeat Timecard Mini"
echo "- Настройки Elasticsearch и мониторинга"
echo "- Все расширенные настройки"
echo ""
echo "Файл fields.yml остается отдельным (как и требовалось)"
echo ""
echo "Для просмотра логов используйте: journalctl -u timebeat -f"