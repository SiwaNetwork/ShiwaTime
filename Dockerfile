# Используем многоступенчатую сборку для минимизации размера образа
FROM golang:1.21-alpine AS builder

# Устанавливаем необходимые пакеты для сборки
RUN apk add --no-cache git make ca-certificates tzdata

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем файлы модулей Go
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN make build

# Финальный минимальный образ
FROM alpine:latest

# Устанавливаем необходимые пакеты
RUN apk --no-cache add ca-certificates tzdata

# Создаем пользователя для запуска приложения
RUN addgroup -g 1001 shiwatime && \
    adduser -D -u 1001 -G shiwatime shiwatime

# Создаем необходимые директории
RUN mkdir -p /app/config /var/log/shiwatime /etc/shiwatime && \
    chown -R shiwatime:shiwatime /app /var/log/shiwatime /etc/shiwatime

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем бинарный файл из builder стадии
COPY --from=builder /app/build/shiwatime /app/shiwatime

# Копируем конфигурационные файлы
COPY --chown=shiwatime:shiwatime config/shiwatime.yml /app/config/

# Переключаемся на непривилегированного пользователя
USER shiwatime

# Открываем порты
EXPOSE 8088 65129

# Переменные окружения
ENV SHIWATIME_CONFIG=/app/config/shiwatime.yml
ENV SHIWATIME_LOG_LEVEL=info

# Healthcheck
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD /app/shiwatime config validate -c $SHIWATIME_CONFIG || exit 1

# Команда запуска
ENTRYPOINT ["/app/shiwatime"]
CMD ["-c", "/app/config/shiwatime.yml"]

# Метаданные
LABEL maintainer="ShiwaTime Team" \
      version="1.0.0" \
      description="ShiwaTime - Time Synchronization Software" \
      org.opencontainers.image.title="ShiwaTime" \
      org.opencontainers.image.description="Time synchronization software with support for NTP, PTP, and other protocols" \
      org.opencontainers.image.version="1.0.0" \
      org.opencontainers.image.source="https://github.com/shiwatime/shiwatime"