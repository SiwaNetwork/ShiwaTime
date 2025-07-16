package protocols

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/shiwatime/shiwatime/internal/config"
)

func TestNewTimeSourceHandler(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Отключаем логи для тестов

	tests := []struct {
		name    string
		config  config.TimeSourceConfig
		wantErr bool
	}{
		{
			name: "NTP TimeSource",
					config: config.TimeSourceConfig{
			Type:           "timesource",
			TimeSourceType: "ntp",
			TimeSourceMode: "client",
			Host:           "192.168.1.100",
			Port:           123,
			Weight:         10,
		},
			wantErr: false,
		},
		{
			name: "PTP TimeSource",
			config: config.TimeSourceConfig{
				Type:           "timesource",
				TimeSourceType: "ptp",
				TimeSourceMode: "slave",
				Interface:      "eth0",
				Weight:         8,
			},
			wantErr: false,
		},
		{
			name: "PPS TimeSource",
			config: config.TimeSourceConfig{
				Type:           "timesource",
				TimeSourceType: "pps",
				TimeSourceMode: "input",
				Device:         "/dev/pps0",
				Weight:         5,
			},
			wantErr: false,
		},
		{
			name: "Mock TimeSource",
			config: config.TimeSourceConfig{
				Type:           "timesource",
				TimeSourceType: "mock",
				TimeSourceMode: "test",
				Weight:         1,
			},
			wantErr: false,
		},
		{
			name: "Invalid Type",
			config: config.TimeSourceConfig{
				Type:           "timesource",
				TimeSourceType: "invalid",
				Weight:         1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := NewTimeSourceHandler(tt.config, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTimeSourceHandler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			// Проверяем, что обработчик создан
			if handler == nil {
				t.Error("NewTimeSourceHandler() returned nil handler")
				return
			}

			// Проверяем конфигурацию
			config := handler.GetConfig()
			if config.Type != tt.config.Type {
				t.Errorf("Config.Type = %v, want %v", config.Type, tt.config.Type)
			}

			// Проверяем статус (должен быть отключен до запуска)
			status := handler.GetStatus()
			if status.Connected {
				t.Error("Handler should not be connected before Start()")
			}
		})
	}
}

func TestTimeSourceHandler_StartStop(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := config.TimeSourceConfig{
		Type:           "timesource",
		TimeSourceType: "mock", // Используем mock для тестирования
		TimeSourceMode: "test",
		Weight:         1,
	}

	handler, err := NewTimeSourceHandler(config, logger)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Тест Start
	t.Run("Start", func(t *testing.T) {
		err := handler.Start()
		if err != nil {
			t.Errorf("Start() error = %v", err)
		}

		// Проверяем статус после запуска
		status := handler.GetStatus()
		if !status.Connected {
			t.Error("Handler should be connected after Start()")
		}
	})

	// Тест Stop
	t.Run("Stop", func(t *testing.T) {
		err := handler.Stop()
		if err != nil {
			t.Errorf("Stop() error = %v", err)
		}

		// Проверяем статус после остановки
		status := handler.GetStatus()
		if status.Connected {
			t.Error("Handler should not be connected after Stop()")
		}
	})
}

func TestTimeSourceHandler_GetTimeInfo(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := config.TimeSourceConfig{
		Type:           "timesource",
		TimesourceType: "mock",
		TimesourceMode: "test",
		Weight:         1,
	}

	handler, err := NewTimeSourceHandler(config, logger)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Запускаем обработчик
	err = handler.Start()
	if err != nil {
		t.Fatalf("Failed to start handler: %v", err)
	}
	defer handler.Stop()

	// Даем время на инициализацию
	time.Sleep(100 * time.Millisecond)

	// Тестируем GetTimeInfo
	timeInfo, err := handler.GetTimeInfo()
	if err != nil {
		t.Errorf("GetTimeInfo() error = %v", err)
		return
	}

	if timeInfo == nil {
		t.Error("GetTimeInfo() returned nil")
		return
	}

	// Проверяем базовые поля
	if timeInfo.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	// Проверяем, что время не слишком старое
	if time.Since(timeInfo.Timestamp) > 5*time.Second {
		t.Error("Timestamp should be recent")
	}
}

func TestTimeSourceHandler_GetStatus(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := config.TimeSourceConfig{
		Type:           "timesource",
		TimesourceType: "mock",
		TimesourceMode: "test",
		Weight:         1,
	}

	handler, err := NewTimeSourceHandler(config, logger)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Тестируем статус до запуска
	status := handler.GetStatus()
	if status.Connected {
		t.Error("Handler should not be connected before Start()")
	}

	// Запускаем обработчик
	err = handler.Start()
	if err != nil {
		t.Fatalf("Failed to start handler: %v", err)
	}
	defer handler.Stop()

	// Даем время на инициализацию
	time.Sleep(100 * time.Millisecond)

	// Тестируем статус после запуска
	status = handler.GetStatus()
	if !status.Connected {
		t.Error("Handler should be connected after Start()")
	}

	// Проверяем, что LastActivity не нулевое
	if status.LastActivity.IsZero() {
		t.Error("LastActivity should not be zero")
	}
}

func TestTimeSourceHandler_ConcurrentAccess(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := config.TimeSourceConfig{
		Type:           "timesource",
		TimesourceType: "mock",
		TimesourceMode: "test",
		Weight:         1,
	}

	handler, err := NewTimeSourceHandler(config, logger)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Запускаем обработчик
	err = handler.Start()
	if err != nil {
		t.Fatalf("Failed to start handler: %v", err)
	}
	defer handler.Stop()

	// Тестируем конкурентный доступ
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			// Конкурентные вызовы GetTimeInfo
			for j := 0; j < 10; j++ {
				_, err := handler.GetTimeInfo()
				if err != nil {
					t.Errorf("Concurrent GetTimeInfo() error = %v", err)
				}
				time.Sleep(1 * time.Millisecond)
			}

			// Конкурентные вызовы GetStatus
			for j := 0; j < 10; j++ {
				status := handler.GetStatus()
				if status.Connected == false {
					t.Error("Handler should remain connected during concurrent access")
				}
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	// Ждем завершения всех горутин
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestTimeSourceHandler_InvalidOperations(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := config.TimeSourceConfig{
		Type:           "timesource",
		TimesourceType: "mock",
		TimesourceMode: "test",
		Weight:         1,
	}

	handler, err := NewTimeSourceHandler(config, logger)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Тестируем GetTimeInfo без запуска
	_, err = handler.GetTimeInfo()
	if err == nil {
		t.Error("GetTimeInfo() should return error when handler is not running")
	}

	// Тестируем повторный Start
	err = handler.Start()
	if err != nil {
		t.Fatalf("First Start() failed: %v", err)
	}

	err = handler.Start()
	if err == nil {
		t.Error("Second Start() should return error")
	}

	// Останавливаем
	handler.Stop()

	// Тестируем повторный Stop
	err = handler.Stop()
	if err != nil {
		t.Errorf("Second Stop() should not return error: %v", err)
	}
}