.PHONY: all build test clean install docker fmt lint deps run run-dev config-validate

# Версионная информация
VERSION ?= 1.0.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Настройки сборки
BINARY_NAME := shiwatime
BUILD_DIR := ./build
CMD_DIR := ./cmd/shiwatime
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

# Go настройки
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# Цели
all: clean deps fmt lint test build

build: ## Сборка бинарного файла
	@echo "Building ShiwaTime $(VERSION) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

build-all: ## Сборка для всех платформ
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	
	@echo "Building for Linux amd64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)
	
	@echo "Building for Linux arm64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)
	
	@echo "Building for Darwin amd64..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	
	@echo "Building for Windows amd64..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)
	
	@echo "Cross-compilation complete!"

test: ## Запуск тестов
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-short: ## Запуск быстрых тестов
	@echo "Running short tests..."
	go test -short -v ./...

bench: ## Запуск бенчмарков
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

clean: ## Очистка собранных файлов
	@echo "Cleaning build directory..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	go clean -testcache

deps: ## Установка зависимостей
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

fmt: ## Форматирование кода
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

lint: ## Проверка кода линтером
	@echo "Running linter..."
	golangci-lint run

install: build ## Установка в GOPATH/bin
	@echo "Installing ShiwaTime..."
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/

run: build ## Запуск с конфигурацией по умолчанию
	@echo "Starting ShiwaTime..."
	$(BUILD_DIR)/$(BINARY_NAME) -c config/shiwatime.yml

run-dev: ## Запуск в режиме разработки
	@echo "Starting ShiwaTime in development mode..."
	go run $(CMD_DIR) -c config/shiwatime.yml -l debug

config-validate: build ## Валидация конфигурации
	@echo "Validating configuration..."
	$(BUILD_DIR)/$(BINARY_NAME) config validate -c config/shiwatime.yml

config-show: build ## Показать текущую конфигурацию
	@echo "Showing current configuration..."
	$(BUILD_DIR)/$(BINARY_NAME) config show -c config/shiwatime.yml

docker: ## Сборка Docker образа
	@echo "Building Docker image..."
	docker build -t shiwatime:$(VERSION) .
	docker tag shiwatime:$(VERSION) shiwatime:latest

docker-run: ## Запуск в Docker контейнере
	@echo "Running ShiwaTime in Docker..."
	docker run --rm -p 8088:8088 -p 65129:65129 \
		-v $(PWD)/config:/app/config \
		shiwatime:latest

setup-dev: ## Настройка среды разработки
	@echo "Setting up development environment..."
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Development tools installed!"

# Создание релиза
release: clean build-all ## Создание релиза
	@echo "Creating release $(VERSION)..."
	@mkdir -p $(BUILD_DIR)/release
	
	# Создание архивов для каждой платформы
	cd $(BUILD_DIR) && tar -czf release/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
	cd $(BUILD_DIR) && tar -czf release/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64
	cd $(BUILD_DIR) && tar -czf release/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64
	cd $(BUILD_DIR) && zip -q release/$(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe
	
	# Создание checksums
	cd $(BUILD_DIR)/release && sha256sum * > checksums.txt
	
	@echo "Release created in $(BUILD_DIR)/release/"

# Утилиты для разработки
tools: ## Установка инструментов разработки
	@echo "Installing development tools..."
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/swaggo/swag/cmd/swag@latest

# Генерация документации
docs: ## Генерация документации
	@echo "Generating documentation..."
	@mkdir -p docs
	go doc -all ./... > docs/api.txt
	@echo "Documentation generated in docs/"

# Мониторинг в реальном времени
monitor: ## Мониторинг логов в реальном времени
	@echo "Monitoring ShiwaTime logs..."
	tail -f /var/log/shiwatime/shiwatime.log 2>/dev/null || echo "Log file not found. Run ShiwaTime first."

# Проверка состояния через API
status: ## Проверка состояния через HTTP API
	@echo "Checking ShiwaTime status..."
	@curl -s http://localhost:8088/api/v1/status | python3 -m json.tool 2>/dev/null || \
		curl -s http://localhost:8088/api/v1/status || \
		echo "ShiwaTime HTTP server not accessible on localhost:8088"

# Тестирование CLI
cli-test: ## Тестирование CLI через SSH
	@echo "Testing CLI interface..."
	@echo "Connect with: ssh -p 65129 admin@localhost"
	@echo "Default password: password"

# Установка systemd сервиса (только для Linux)
install-service: build ## Установка systemd сервиса
	@echo "Installing systemd service..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	sudo mkdir -p /etc/shiwatime
	sudo cp config/shiwatime.yml /etc/shiwatime/
	
	@echo "[Unit]" | sudo tee /etc/systemd/system/shiwatime.service
	@echo "Description=ShiwaTime - Time Synchronization Service" | sudo tee -a /etc/systemd/system/shiwatime.service
	@echo "After=network.target" | sudo tee -a /etc/systemd/system/shiwatime.service
	@echo "" | sudo tee -a /etc/systemd/system/shiwatime.service
	@echo "[Service]" | sudo tee -a /etc/systemd/system/shiwatime.service
	@echo "Type=simple" | sudo tee -a /etc/systemd/system/shiwatime.service
	@echo "User=shiwatime" | sudo tee -a /etc/systemd/system/shiwatime.service
	@echo "Group=shiwatime" | sudo tee -a /etc/systemd/system/shiwatime.service
	@echo "ExecStart=/usr/local/bin/$(BINARY_NAME) -c /etc/shiwatime/shiwatime.yml" | sudo tee -a /etc/systemd/system/shiwatime.service
	@echo "Restart=always" | sudo tee -a /etc/systemd/system/shiwatime.service
	@echo "RestartSec=5" | sudo tee -a /etc/systemd/system/shiwatime.service
	@echo "" | sudo tee -a /etc/systemd/system/shiwatime.service
	@echo "[Install]" | sudo tee -a /etc/systemd/system/shiwatime.service
	@echo "WantedBy=multi-user.target" | sudo tee -a /etc/systemd/system/shiwatime.service
	
	sudo systemctl daemon-reload
	@echo "Service installed. Enable with: sudo systemctl enable shiwatime"
	@echo "Start with: sudo systemctl start shiwatime"

help: ## Показать справку
	@echo "ShiwaTime - Time Synchronization Software"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# По умолчанию показывать справку
.DEFAULT_GOAL := help