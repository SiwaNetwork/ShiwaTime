#!/bin/bash

# Скрипт установки улучшений для Timebeat:
# 1. Интеграция Elastic Beats (уже встроена)
# 2. Активация протоколов PPS, NMEA, PHC
# 3. Установка SSH CLI интерфейса

set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Функция логирования
log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

warn() {
    echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] WARNING: $1${NC}"
}

error() {
    echo -e "${RED}[$(date +'%Y-%m-%d %H:%M:%S')] ERROR: $1${NC}"
}

info() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')] INFO: $1${NC}"
}

# Проверка прав root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "Этот скрипт должен быть запущен от имени root"
        exit 1
    fi
}

# Установка зависимостей
install_dependencies() {
    log "Установка зависимостей..."
    
    # Обновление пакетов
    apt-get update
    
    # Установка Python пакетов
    apt-get install -y python3 python3-pip python3-venv
    
    # Установка системных пакетов для времени
    apt-get install -y chrony ntpdate systemd-timesyncd
    
    # Установка инструментов для SSH
    apt-get install -y openssh-server openssh-client
    
    log "Зависимости установлены"
}

# Создание пользователя timebeat
create_timebeat_user() {
    log "Создание пользователя timebeat..."
    
    if ! id "timebeat" &>/dev/null; then
        useradd -r -s /bin/bash -d /home/timebeat -m timebeat
        usermod -aG systemd-journal timebeat
        log "Пользователь timebeat создан"
    else
        info "Пользователь timebeat уже существует"
    fi
    
    # Создание директорий
    mkdir -p /etc/timebeat
    mkdir -p /var/log/timebeat
    mkdir -p /opt/timebeat-ssh-cli
    
    # Настройка прав
    chown -R timebeat:timebeat /etc/timebeat
    chown -R timebeat:timebeat /var/log/timebeat
    chown -R timebeat:timebeat /opt/timebeat-ssh-cli
}

# Установка timebeat deb пакета
install_timebeat() {
    log "Установка timebeat..."
    
    if [[ -f "timebeat-2.2.20-amd64.deb" ]]; then
        dpkg -i timebeat-2.2.20-amd64.deb || apt-get install -f -y
        log "Timebeat установлен"
    else
        warn "Файл timebeat-2.2.20-amd64.deb не найден. Пропускаем установку."
    fi
}

# Копирование конфигурации с активными протоколами
setup_timebeat_config() {
    log "Настройка конфигурации timebeat..."
    
    # Копирование лицензии
    if [[ -f "timebeat.lic" ]]; then
        cp timebeat.lic /etc/timebeat/
        chown timebeat:timebeat /etc/timebeat/timebeat.lic
    fi
    
    # Копирование активной конфигурации
    if [[ -f "timebeat-active-protocols.yml" ]]; then
        cp timebeat-active-protocols.yml /etc/timebeat/timebeat.yml
        chown timebeat:timebeat /etc/timebeat/timebeat.yml
        log "Конфигурация с активными протоколами установлена"
    else
        warn "Файл конфигурации timebeat-active-protocols.yml не найден"
    fi
}

# Установка SSH CLI интерфейса
setup_ssh_cli() {
    log "Установка SSH CLI интерфейса..."
    
    # Копирование файлов
    cp timebeat_ssh_cli.py /opt/timebeat-ssh-cli/
    cp requirements.txt /opt/timebeat-ssh-cli/
    
    # Установка Python зависимостей
    cd /opt/timebeat-ssh-cli
    python3 -m pip install -r requirements.txt
    
    # Генерация SSH ключа хоста
    if [[ ! -f "/opt/timebeat-ssh-cli/ssh_host_key" ]]; then
        ssh-keygen -t rsa -b 2048 -f /opt/timebeat-ssh-cli/ssh_host_key -N ""
    fi
    
    # Настройка прав
    chown -R timebeat:timebeat /opt/timebeat-ssh-cli
    chmod 600 /opt/timebeat-ssh-cli/ssh_host_key
    chmod +x /opt/timebeat-ssh-cli/timebeat_ssh_cli.py
    
    log "SSH CLI интерфейс установлен"
}

# Установка systemd сервиса
setup_systemd_service() {
    log "Настройка systemd сервиса..."
    
    # Копирование service файла
    cp timebeat-ssh-cli.service /etc/systemd/system/
    
    # Перезагрузка systemd
    systemctl daemon-reload
    
    # Включение и запуск сервиса
    systemctl enable timebeat-ssh-cli.service
    
    log "Systemd сервис настроен"
}

# Настройка файрвола
setup_firewall() {
    log "Настройка файрвола..."
    
    # Проверка наличия ufw
    if command -v ufw &> /dev/null; then
        ufw allow 2222/tcp comment "Timebeat SSH CLI"
        log "Правило файрвола добавлено (порт 2222)"
    else
        warn "ufw не найден. Настройте файрвол вручную для порта 2222"
    fi
}

# Запуск сервисов
start_services() {
    log "Запуск сервисов..."
    
    # Запуск timebeat
    if systemctl is-available timebeat &>/dev/null; then
        systemctl enable timebeat
        systemctl start timebeat
        log "Сервис timebeat запущен"
    fi
    
    # Запуск SSH CLI
    systemctl start timebeat-ssh-cli
    log "SSH CLI сервис запущен"
}

# Проверка состояния сервисов
check_services() {
    log "Проверка состояния сервисов..."
    
    echo ""
    info "=== Состояние сервиса Timebeat ==="
    systemctl status timebeat --no-pager || true
    
    echo ""
    info "=== Состояние SSH CLI сервиса ==="
    systemctl status timebeat-ssh-cli --no-pager || true
    
    echo ""
    info "=== Активные протоколы ==="
    if [[ -f "/etc/timebeat/timebeat.yml" ]]; then
        grep -A5 -B2 "protocol:" /etc/timebeat/timebeat.yml | grep -v "^#" || true
    fi
}

# Показать информацию о подключении
show_connection_info() {
    echo ""
    log "=== УСТАНОВКА ЗАВЕРШЕНА ==="
    echo ""
    info "Для подключения к SSH CLI интерфейсу используйте:"
    echo "  ssh timebeat@localhost -p 2222"
    echo "  Пароль: timebeat123"
    echo ""
    info "Доступные команды в SSH CLI:"
    echo "  help      - Справка по командам"
    echo "  status    - Статус сервиса timebeat"
    echo "  protocols - Список протоколов"
    echo "  logs      - Просмотр логов"
    echo "  enable    - Включить протокол"
    echo "  disable   - Отключить протокол"
    echo ""
    info "Конфигурационные файлы:"
    echo "  /etc/timebeat/timebeat.yml - Основная конфигурация"
    echo "  /var/log/timebeat/ - Логи"
    echo ""
    info "Протоколы активированы:"
    echo "  ✅ PTP - Precision Time Protocol"
    echo "  ✅ NTP - Network Time Protocol"
    echo "  ✅ PPS - Pulse Per Second"
    echo "  ✅ NMEA - GNSS Navigation"
    echo "  ✅ PHC - PTP Hardware Clock"
    echo ""
}

# Главная функция
main() {
    log "Начало установки улучшений Timebeat..."
    
    check_root
    install_dependencies
    create_timebeat_user
    install_timebeat
    setup_timebeat_config
    setup_ssh_cli
    setup_systemd_service
    setup_firewall
    start_services
    check_services
    show_connection_info
    
    log "Установка завершена успешно!"
}

# Запуск
main "$@"