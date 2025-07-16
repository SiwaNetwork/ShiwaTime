#!/bin/bash

# Скрипт для настройки интеграции с management-platform
# https://github.com/timebeat-app/management-platform

set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Функции для вывода
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Проверка прав администратора
check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "Этот скрипт должен выполняться с правами администратора"
        log_info "Запустите: sudo $0"
        exit 1
    fi
}

# Создание резервной копии конфигурации
backup_config() {
    log_info "Создание резервной копии конфигурации..."
    
    local backup_dir="/etc/timebeat/backup/$(date +%Y%m%d_%H%M%S)"
    mkdir -p "$backup_dir"
    
    if [ -f "/etc/timebeat/timebeat.yml" ]; then
        cp "/etc/timebeat/timebeat.yml" "$backup_dir/"
        log_success "Резервная копия создана: $backup_dir/timebeat.yml"
    fi
    
    if [ -f "timebeat.yml" ]; then
        cp "timebeat.yml" "$backup_dir/"
        log_success "Резервная копия создана: $backup_dir/timebeat.yml (локальный)"
    fi
}

# Настройка конфигурации для management-platform
setup_management_config() {
    log_info "Настройка конфигурации для management-platform..."
    
    # Создаем директорию для конфигурации
    mkdir -p /etc/timebeat/pki
    
    # Создаем конфигурационный файл для cloud
    cat > /etc/timebeat/timebeat-cloud.yml << 'EOF'
# Конфигурация для интеграции с management-platform
timebeat:
  license:
    keyfile: '/etc/timebeat/timebeat.lic'
  
  clock_sync:
    adjust_clock: true
    step_limit: 15m
    
    primary_clocks:
      # PTP источник
      - protocol: ptp
        domain: 0
        serve_unicast: true
        serve_multicast: true
        server_only: true
        announce_interval: 1
        sync_interval: 0
        delayrequest_interval: 0
        disable: false
        interface: enp0s3  # Измените на вашу сетевую карту
        
      # NTP fallback
      - protocol: ntp
        ip: 'pool.ntp.org'
        pollinterval: 4s
        monitor_only: false

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

  logging:
    stdout.enable: true
    to_files: true
    files:
      path: /var/log/timebeat
      name: timebeat
      rotateeverybytes: 10485760
      keepfiles: 7

# Elasticsearch output для management-platform
output.elasticsearch:
  hosts: ['elastic.customer.timebeat.app:9200']
  protocol: 'https'
  
  # Аутентификация (замените на ваши данные)
  username: 'elastic'
  password: 'changeme'
  
  # SSL сертификаты
  ssl.certificate_authorities: ['/etc/timebeat/pki/ca.crt']
  ssl.certificate: '/etc/timebeat/pki/timebeat.crt'
  ssl.key: '/etc/timebeat/pki/timebeat.key'
  ssl.verification_mode: "certificate"

# Настройки мониторинга
monitoring.enabled: true
monitoring.elasticsearch:

# Index Lifecycle Management
setup.ilm.enabled: true
setup.ilm.policy_name: "timebeat"
setup.ilm.check_exists: true
setup.ilm.rollover_alias: "timebeat"

# Логирование
logging.to_files: true
logging.files:
  path: /var/log/timebeat
  name: timebeat
  rotateeverybytes: 10485760
  keepfiles: 7
  permissions: 0600

# Безопасность
seccomp.enabled: false
EOF

    log_success "Конфигурация создана: /etc/timebeat/timebeat-cloud.yml"
}

# Создание SSL сертификатов (self-signed для тестирования)
create_ssl_certificates() {
    log_info "Создание SSL сертификатов..."
    
    local cert_dir="/etc/timebeat/pki"
    mkdir -p "$cert_dir"
    
    # Создаем CA сертификат
    openssl req -x509 -newkey rsa:4096 -keyout "$cert_dir/ca.key" -out "$cert_dir/ca.crt" \
        -days 365 -nodes -subj "/C=US/ST=State/L=City/O=Timebeat/CN=Timebeat CA" 2>/dev/null || {
        log_warning "Не удалось создать CA сертификат. Установите openssl"
        return 1
    }
    
    # Создаем сертификат клиента
    openssl req -newkey rsa:4096 -keyout "$cert_dir/timebeat.key" -out "$cert_dir/timebeat.csr" \
        -nodes -subj "/C=US/ST=State/L=City/O=Timebeat/CN=timebeat-client" 2>/dev/null || {
        log_warning "Не удалось создать CSR"
        return 1
    }
    
    # Подписываем сертификат
    openssl x509 -req -in "$cert_dir/timebeat.csr" -CA "$cert_dir/ca.crt" -CAkey "$cert_dir/ca.key" \
        -CAcreateserial -out "$cert_dir/timebeat.crt" -days 365 2>/dev/null || {
        log_warning "Не удалось подписать сертификат"
        return 1
    }
    
    # Устанавливаем права
    chmod 600 "$cert_dir"/*.key
    chmod 644 "$cert_dir"/*.crt
    
    log_success "SSL сертификаты созданы в $cert_dir"
}

# Настройка systemd сервиса
setup_systemd_service() {
    log_info "Настройка systemd сервиса..."
    
    # Создаем пользователя
    if ! id "timebeat" &>/dev/null; then
        useradd -r -s /bin/false -d /etc/timebeat timebeat
        log_success "Пользователь timebeat создан"
    fi
    
    # Создаем systemd сервис
    cat > /etc/systemd/system/timebeat-cloud.service << 'EOF'
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

# Требуемые возможности
CapabilityBoundingSet=CAP_SYS_TIME CAP_NET_ADMIN CAP_NET_RAW
AmbientCapabilities=CAP_SYS_TIME CAP_NET_ADMIN CAP_NET_RAW

# Безопасность
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/timebeat /etc/timebeat

[Install]
WantedBy=multi-user.target
EOF

    # Перезагружаем systemd
    systemctl daemon-reload
    
    log_success "Systemd сервис создан: timebeat-cloud.service"
}

# Настройка firewall
setup_firewall() {
    log_info "Настройка firewall..."
    
    # Проверяем наличие ufw или iptables
    if command -v ufw &> /dev/null; then
        ufw allow 8088/tcp comment "Timebeat HTTP API"
        ufw allow 65129/tcp comment "Timebeat SSH CLI"
        log_success "UFW правила добавлены"
    elif command -v iptables &> /dev/null; then
        iptables -A INPUT -p tcp --dport 8088 -j ACCEPT
        iptables -A INPUT -p tcp --dport 65129 -j ACCEPT
        log_success "IPTables правила добавлены"
    else
        log_warning "Firewall не найден (ufw/iptables)"
    fi
}

# Создание тестового лицензионного файла
create_test_license() {
    log_info "Создание тестовой лицензии..."
    
    cat > /etc/timebeat/timebeat.lic << 'EOF'
# Тестовая лицензия для development
# В продакшене замените на реальную лицензию от timebeat.app

LICENSE_TYPE=development
CUSTOMER_ID=test-customer
EXPIRY_DATE=2025-12-31
FEATURES=ptp,ntp,pps,phc,cloud
EOF

    chown timebeat:timebeat /etc/timebeat/timebeat.lic
    chmod 600 /etc/timebeat/timebeat.lic
    
    log_success "Тестовая лицензия создана"
}

# Настройка логирования
setup_logging() {
    log_info "Настройка логирования..."
    
    mkdir -p /var/log/timebeat
    chown timebeat:timebeat /var/log/timebeat
    chmod 755 /var/log/timebeat
    
    # Создаем logrotate конфигурацию
    cat > /etc/logrotate.d/timebeat << 'EOF'
/var/log/timebeat/*.log {
    daily
    missingok
    rotate 7
    compress
    delaycompress
    notifempty
    create 644 timebeat timebeat
    postrotate
        systemctl reload timebeat-cloud.service 2>/dev/null || true
    endscript
}
EOF

    log_success "Логирование настроено"
}

# Создание тестового скрипта
create_test_script() {
    log_info "Создание тестового скрипта..."
    
    cat > /usr/local/bin/test-timebeat-cloud.sh << 'EOF'
#!/bin/bash

echo "=== Тестирование Timebeat Cloud ==="

# Проверка сервиса
echo "1. Проверка systemd сервиса..."
systemctl status timebeat-cloud.service --no-pager

# Проверка HTTP API
echo -e "\n2. Проверка HTTP API..."
curl -s http://localhost:8088/api/v1/status | jq . 2>/dev/null || echo "HTTP API недоступен"

# Проверка метрик
echo -e "\n3. Проверка метрик..."
curl -s http://localhost:8088/metrics | head -10

# Проверка SSH CLI
echo -e "\n4. Проверка SSH CLI..."
nc -z localhost 65129 && echo "SSH CLI доступен" || echo "SSH CLI недоступен"

# Проверка логов
echo -e "\n5. Последние логи..."
tail -5 /var/log/timebeat/timebeat 2>/dev/null || echo "Логи не найдены"

echo -e "\n=== Тестирование завершено ==="
EOF

    chmod +x /usr/local/bin/test-timebeat-cloud.sh
    log_success "Тестовый скрипт создан: /usr/local/bin/test-timebeat-cloud.sh"
}

# Основная функция
main() {
    echo "=========================================="
    echo "Настройка интеграции с management-platform"
    echo "=========================================="
    echo
    
    check_root
    
    backup_config
    echo
    
    setup_management_config
    echo
    
    create_ssl_certificates
    echo
    
    setup_systemd_service
    echo
    
    setup_firewall
    echo
    
    create_test_license
    echo
    
    setup_logging
    echo
    
    create_test_script
    echo
    
    echo "=========================================="
    log_success "Настройка завершена!"
    echo "=========================================="
    echo
    echo "Следующие шаги:"
    echo "1. Отредактируйте /etc/timebeat/timebeat-cloud.yml"
    echo "2. Замените тестовую лицензию на реальную"
    echo "3. Настройте SSL сертификаты для продакшена"
    echo "4. Запустите сервис: systemctl enable --now timebeat-cloud.service"
    echo "5. Проверьте работу: /usr/local/bin/test-timebeat-cloud.sh"
    echo
}

# Запуск основной функции
main "$@"