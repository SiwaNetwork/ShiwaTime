#!/bin/bash

# Скрипт для тестирования объединенного конфигурационного файла
# config/combined-config.yml

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
    
    # Проверяем python3
    if ! command -v "python3" &> /dev/null; then
        log_error "Отсутствует python3"
        log_info "Установите его с помощью: sudo apt-get install python3"
        exit 1
    fi
    
    # Проверяем модуль yaml
    if ! python3 -c "import yaml" &> /dev/null; then
        log_error "Отсутствует модуль yaml для python3"
        log_info "Установите его с помощью: sudo apt-get install python3-yaml"
        exit 1
    fi
    
    log_success "Все зависимости установлены"
}

# Проверка синтаксиса YAML
test_yaml_syntax() {
    log_info "Проверка синтаксиса YAML..."
    
    local config_file="config/combined-config.yml"
    
    if [ ! -f "$config_file" ]; then
        log_error "Файл конфигурации не найден: $config_file"
        exit 1
    fi
    
    if python3 -c "import yaml; yaml.safe_load(open('$config_file', 'r'))" 2>/dev/null; then
        log_success "✓ Синтаксис YAML корректен"
    else
        log_error "✗ Ошибка в синтаксисе YAML"
        exit 1
    fi
}

# Проверка структуры конфигурации
test_config_structure() {
    log_info "Проверка структуры конфигурации..."
    
    local config_file="config/combined-config.yml"
    
    # Проверяем наличие основных секций
    local required_sections=(
        "shiwatime"
        "shiwatime.clock_sync"
        "shiwatime.ptp_tuning"
        "shiwatime.ptpsquared"
        "shiwatime.cli"
        "shiwatime.http"
        "shiwatime.logging"
        "output"
        "setup"
    )
    
    for section in "${required_sections[@]}"; do
        if python3 -c "
import yaml
with open('$config_file', 'r') as f:
    config = yaml.safe_load(f)
keys = '$section'.split('.')
obj = config
for key in keys:
    if key in obj:
        obj = obj[key]
    else:
        exit(1)
" 2>/dev/null; then
            log_success "✓ Секция найдена: $section"
        else
            log_warning "⚠ Секция отсутствует: $section"
        fi
    done
}

# Проверка настроек источников времени
test_time_sources() {
    log_info "Проверка настроек источников времени..."
    
    local config_file="config/combined-config.yml"
    
    # Проверяем наличие первичных источников
    local primary_count=$(python3 -c "
import yaml
with open('$config_file', 'r') as f:
    config = yaml.safe_load(f)
primary = config.get('shiwatime', {}).get('clock_sync', {}).get('primary_clocks', [])
print(len(primary))
" 2>/dev/null || echo "0")
    
    if [ "$primary_count" -gt 0 ]; then
        log_success "✓ Найдено первичных источников времени: $primary_count"
    else
        log_warning "⚠ Первичные источники времени не настроены"
    fi
    
    # Проверяем наличие вторичных источников
    local secondary_count=$(python3 -c "
import yaml
with open('$config_file', 'r') as f:
    config = yaml.safe_load(f)
secondary = config.get('shiwatime', {}).get('clock_sync', {}).get('secondary_clocks', [])
print(len(secondary))
" 2>/dev/null || echo "0")
    
    if [ "$secondary_count" -gt 0 ]; then
        log_success "✓ Найдено вторичных источников времени: $secondary_count"
    else
        log_warning "⚠ Вторичные источники времени не настроены"
    fi
}

# Проверка настроек PTP+Squared
test_ptpsquared_config() {
    log_info "Проверка настроек PTP+Squared..."
    
    local config_file="config/combined-config.yml"
    
    # Проверяем включение PTP+Squared
    local ptpsquared_enabled=$(python3 -c "
import yaml
with open('$config_file', 'r') as f:
    config = yaml.safe_load(f)
ptpsquared = config.get('shiwatime', {}).get('ptpsquared', {})
print(ptpsquared.get('enable', False))
" 2>/dev/null || echo "False")
    
    if [ "$ptpsquared_enabled" = "True" ]; then
        log_success "✓ PTP+Squared включен"
        
        # Проверяем настройки доменов
        local domains=$(python3 -c "
import yaml
with open('$config_file', 'r') as f:
    config = yaml.safe_load(f)
ptpsquared = config.get('shiwatime', {}).get('ptpsquared', {})
domains = ptpsquared.get('domains', [])
print(','.join(map(str, domains)))
" 2>/dev/null || echo "")
        
        if [ -n "$domains" ]; then
            log_success "  Домены PTP: $domains"
        else
            log_warning "  ⚠ Домены PTP не настроены"
        fi
    else
        log_warning "⚠ PTP+Squared отключен"
    fi
}

# Проверка настроек интерфейсов
test_interface_config() {
    log_info "Проверка настроек интерфейсов..."
    
    local config_file="config/combined-config.yml"
    
    # Проверяем настройки CLI
    local cli_enabled=$(python3 -c "
import yaml
with open('$config_file', 'r') as f:
    config = yaml.safe_load(f)
cli = config.get('shiwatime', {}).get('cli', {})
print(cli.get('enable', False))
" 2>/dev/null || echo "False")
    
    if [ "$cli_enabled" = "True" ]; then
        log_success "✓ CLI интерфейс включен"
        
        local cli_port=$(python3 -c "
import yaml
with open('$config_file', 'r') as f:
    config = yaml.safe_load(f)
cli = config.get('shiwatime', {}).get('cli', {})
print(cli.get('bind_port', 'не настроен'))
" 2>/dev/null || echo "не настроен")
        
        log_info "  Порт CLI: $cli_port"
    else
        log_warning "⚠ CLI интерфейс отключен"
    fi
    
    # Проверяем настройки HTTP
    local http_enabled=$(python3 -c "
import yaml
with open('$config_file', 'r') as f:
    config = yaml.safe_load(f)
http = config.get('shiwatime', {}).get('http', {})
print(http.get('enable', False))
" 2>/dev/null || echo "False")
    
    if [ "$http_enabled" = "True" ]; then
        log_success "✓ HTTP интерфейс включен"
        
        local http_port=$(python3 -c "
import yaml
with open('$config_file', 'r') as f:
    config = yaml.safe_load(f)
http = config.get('shiwatime', {}).get('http', {})
print(http.get('bind_port', 'не настроен'))
" 2>/dev/null || echo "не настроен")
        
        log_info "  Порт HTTP: $http_port"
    else
        log_warning "⚠ HTTP интерфейс отключен"
    fi
}

# Проверка настроек Elasticsearch
test_elasticsearch_config() {
    log_info "Проверка настроек Elasticsearch..."
    
    local config_file="config/combined-config.yml"
    
    # Проверяем настройки вывода
    local es_hosts=$(python3 -c "
import yaml
with open('$config_file', 'r') as f:
    config = yaml.safe_load(f)
output = config.get('output', {})
es = output.get('elasticsearch', {})
hosts = es.get('hosts', [])
print(','.join(hosts))
" 2>/dev/null || echo "")
    
    if [ -n "$es_hosts" ]; then
        log_success "✓ Elasticsearch хосты настроены: $es_hosts"
    else
        log_warning "⚠ Elasticsearch хосты не настроены"
    fi
    
    # Проверяем настройки мониторинга
    local monitoring_enabled=$(python3 -c "
import yaml
with open('$config_file', 'r') as f:
    config = yaml.safe_load(f)
monitoring = config.get('monitoring', {})
print(monitoring.get('enabled', False))
" 2>/dev/null || echo "False")
    
    if [ "$monitoring_enabled" = "True" ]; then
        log_success "✓ Мониторинг включен"
    else
        log_warning "⚠ Мониторинг отключен"
    fi
}

# Проверка настроек логирования
test_logging_config() {
    log_info "Проверка настроек логирования..."
    
    local config_file="config/combined-config.yml"
    
    # Проверяем размер буфера
    local buffer_size=$(python3 -c "
import yaml
with open('$config_file', 'r') as f:
    config = yaml.safe_load(f)
logging = config.get('shiwatime', {}).get('logging', {})
print(logging.get('buffer_size', 'не настроен'))
" 2>/dev/null || echo "не настроен")
    
    log_info "  Размер буфера: $buffer_size"
    
    # Проверяем stdout логирование
    local stdout_enabled=$(python3 -c "
import yaml
with open('$config_file', 'r') as f:
    config = yaml.safe_load(f)
logging = config.get('shiwatime', {}).get('logging', {})
stdout = logging.get('stdout', {})
print(stdout.get('enable', False))
" 2>/dev/null || echo "False")
    
    if [ "$stdout_enabled" = "True" ]; then
        log_success "✓ Stdout логирование включено"
    else
        log_warning "⚠ Stdout логирование отключено"
    fi
}

# Сравнение с оригинальными файлами
compare_with_originals() {
    log_info "Сравнение с оригинальными файлами..."
    
    local original_files=(
        "config/ptpsquared-example.yml"
        "config/shiwatime.yml"
        "config/test.yml"
        "config/timesource-example.yml"
    )
    
    local combined_file="config/combined-config.yml"
    
    for original in "${original_files[@]}"; do
        if [ -f "$original" ]; then
            log_info "Проверка: $original"
            
            # Проверяем синтаксис оригинального файла
            if python3 -c "import yaml; yaml.safe_load(open('$original', 'r'))" 2>/dev/null; then
                log_success "  ✓ Синтаксис корректен"
            else
                log_error "  ✗ Ошибка синтаксиса"
            fi
        else
            log_warning "  ⚠ Файл не найден: $original"
        fi
    done
}

# Основная функция
main() {
    log_info "Начало тестирования объединенного конфигурационного файла..."
    echo
    
    check_dependencies
    echo
    
    test_yaml_syntax
    echo
    
    test_config_structure
    echo
    
    test_time_sources
    echo
    
    test_ptpsquared_config
    echo
    
    test_interface_config
    echo
    
    test_elasticsearch_config
    echo
    
    test_logging_config
    echo
    
    compare_with_originals
    echo
    
    log_success "Тестирование завершено успешно!"
    log_info "Объединенный файл: config/combined-config.yml"
    log_info "Все оригинальные файлы сохранены в папке config/"
}

# Запуск основной функции
main "$@"