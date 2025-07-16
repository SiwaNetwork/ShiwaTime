#!/bin/bash

# Скрипт для тестирования интеграции с management-platform
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

# Проверка зависимостей
check_dependencies() {
    log_info "Проверка зависимостей..."
    
    local deps=("curl" "jq" "nc" "timeout")
    local missing=()
    
    for dep in "${deps[@]}"; do
        if ! command -v "$dep" &> /dev/null; then
            missing+=("$dep")
        fi
    done
    
    if [ ${#missing[@]} -ne 0 ]; then
        log_error "Отсутствуют зависимости: ${missing[*]}"
        log_info "Установите их с помощью: sudo apt-get install curl jq netcat-openbsd"
        exit 1
    fi
    
    log_success "Все зависимости установлены"
}

# Проверка конфигурации
check_configuration() {
    log_info "Проверка конфигурации..."
    
    local config_files=("timebeat.yml" "config/shiwatime.yml")
    
    for config in "${config_files[@]}"; do
        if [ -f "$config" ]; then
            log_success "Найден файл конфигурации: $config"
            
            # Проверяем настройки Elasticsearch
            if grep -q "elastic.customer.timebeat.app" "$config"; then
                log_warning "Обнаружена конфигурация timebeat cloud в $config"
            fi
            
            # Проверяем настройки мониторинга
            if grep -q "monitoring.enabled: true" "$config"; then
                log_success "Мониторинг включен в $config"
            else
                log_warning "Мониторинг отключен в $config"
            fi
        else
            log_warning "Файл конфигурации не найден: $config"
        fi
    done
}

# Проверка HTTP API
test_http_api() {
    log_info "Тестирование HTTP API..."
    
    local base_url="http://localhost:8088"
    local endpoints=(
        "/api/v1/status"
        "/api/v1/sources"
        "/api/v1/health"
        "/metrics"
    )
    
    for endpoint in "${endpoints[@]}"; do
        local url="$base_url$endpoint"
        log_info "Тестирование: $url"
        
        if timeout 5 curl -s "$url" > /dev/null 2>&1; then
            log_success "✓ $endpoint доступен"
            
            # Получаем и анализируем ответ
            local response=$(timeout 5 curl -s "$url")
            if [ -n "$response" ]; then
                if echo "$response" | jq . > /dev/null 2>&1; then
                    log_success "  Ответ в формате JSON"
                else
                    log_success "  Ответ получен (не JSON)"
                fi
            else
                log_warning "  Пустой ответ"
            fi
        else
            log_error "✗ $endpoint недоступен"
        fi
    done
}

# Проверка метрик Prometheus
test_prometheus_metrics() {
    log_info "Тестирование метрик Prometheus..."
    
    local metrics_url="http://localhost:8088/metrics"
    
    if timeout 5 curl -s "$metrics_url" > /dev/null 2>&1; then
        log_success "✓ Метрики доступны"
        
        local metrics=$(timeout 5 curl -s "$metrics_url")
        
        # Проверяем наличие ключевых метрик
        local expected_metrics=(
            "shiwatime_clock_state"
            "shiwatime_sources_total"
            "shiwatime_sources_active"
        )
        
        for metric in "${expected_metrics[@]}"; do
            if echo "$metrics" | grep -q "$metric"; then
                log_success "  ✓ Найдена метрика: $metric"
            else
                log_warning "  ⚠ Метрика не найдена: $metric"
            fi
        done
        
        # Выводим все доступные метрики
        log_info "Доступные метрики:"
        echo "$metrics" | grep -E "^[a-zA-Z_]" | head -10
    else
        log_error "✗ Метрики недоступны"
    fi
}

# Проверка SSH CLI
test_ssh_cli() {
    log_info "Тестирование SSH CLI..."
    
    local ssh_port=65129
    
    if timeout 5 nc -z localhost "$ssh_port" 2>/dev/null; then
        log_success "✓ SSH CLI доступен на порту $ssh_port"
        
        # Пытаемся подключиться и выполнить команду
        if timeout 10 ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 \
            -p "$ssh_port" admin@localhost "help" 2>/dev/null; then
            log_success "  ✓ SSH CLI работает корректно"
        else
            log_warning "  ⚠ SSH CLI доступен, но команды не выполняются"
        fi
    else
        log_error "✗ SSH CLI недоступен на порту $ssh_port"
    fi
}

# Проверка Elasticsearch интеграции
test_elasticsearch_integration() {
    log_info "Тестирование интеграции с Elasticsearch..."
    
    # Проверяем подключение к Elasticsearch
    local es_hosts=("localhost:9200" "elastic.customer.timebeat.app:9200")
    
    for host in "${es_hosts[@]}"; do
        log_info "Проверка подключения к: $host"
        
        if timeout 5 curl -s "http://$host" > /dev/null 2>&1; then
            log_success "✓ Elasticsearch доступен: $host"
            
            # Проверяем индексы
            local indices=$(timeout 5 curl -s "http://$host/_cat/indices?v" 2>/dev/null || echo "")
            if [ -n "$indices" ]; then
                log_success "  Индексы Elasticsearch:"
                echo "$indices" | head -5
            fi
        else
            log_warning "⚠ Elasticsearch недоступен: $host"
        fi
    done
}

# Проверка systemd сервисов
test_systemd_services() {
    log_info "Проверка systemd сервисов..."
    
    local services=("timebeat" "timebeat-ssh-cli")
    
    for service in "${services[@]}"; do
        if systemctl is-active --quiet "$service" 2>/dev/null; then
            log_success "✓ Сервис $service активен"
            
            # Проверяем статус
            local status=$(systemctl is-enabled "$service" 2>/dev/null || echo "unknown")
            log_info "  Статус: $status"
        else
            log_warning "⚠ Сервис $service не активен"
        fi
    done
}

# Проверка логов
check_logs() {
    log_info "Проверка логов..."
    
    local log_files=(
        "/var/log/timebeat/timebeat"
        "/var/log/shiwatime/shiwatime.log"
    )
    
    for log_file in "${log_files[@]}"; do
        if [ -f "$log_file" ]; then
            log_success "✓ Найден лог файл: $log_file"
            
            # Проверяем последние записи
            local recent_logs=$(tail -5 "$log_file" 2>/dev/null || echo "")
            if [ -n "$recent_logs" ]; then
                log_info "  Последние записи:"
                echo "$recent_logs" | head -3
            fi
        else
            log_warning "⚠ Лог файл не найден: $log_file"
        fi
    done
}

# Проверка интеграции с management-platform
test_management_platform_integration() {
    log_info "Тестирование интеграции с management-platform..."
    
    # Проверяем конфигурацию для cloud
    if grep -q "elastic.customer.timebeat.app" timebeat.yml; then
        log_success "✓ Обнаружена конфигурация timebeat cloud"
        
        # Проверяем SSL сертификаты
        local cert_files=(
            "/etc/timebeat/pki/ca.crt"
            "/etc/timebeat/pki/timebeat.crt"
            "/etc/timebeat/pki/timebeat.key"
        )
        
        for cert in "${cert_files[@]}"; do
            if [ -f "$cert" ]; then
                log_success "  ✓ Найден сертификат: $cert"
            else
                log_warning "  ⚠ Сертификат не найден: $cert"
            fi
        done
    else
        log_warning "⚠ Конфигурация timebeat cloud не найдена"
    fi
    
    # Проверяем лицензию
    if [ -f "timebeat.lic" ]; then
        log_success "✓ Найдена лицензия timebeat"
    else
        log_warning "⚠ Лицензия timebeat не найдена"
    fi
}

# Основная функция
main() {
    echo "=========================================="
    echo "Тестирование интеграции с management-platform"
    echo "=========================================="
    echo
    
    check_dependencies
    echo
    
    check_configuration
    echo
    
    test_http_api
    echo
    
    test_prometheus_metrics
    echo
    
    test_ssh_cli
    echo
    
    test_elasticsearch_integration
    echo
    
    test_systemd_services
    echo
    
    check_logs
    echo
    
    test_management_platform_integration
    echo
    
    echo "=========================================="
    log_success "Тестирование завершено!"
    echo "=========================================="
}

# Запуск основной функции
main "$@"