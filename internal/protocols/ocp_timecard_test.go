package protocols

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/shiwatime/shiwatime/internal/config"
)

func TestNewOCPTimecardHandler(t *testing.T) {
	logger := logrus.New()
	
	config := config.TimeSourceConfig{
		Type:           "ocp_timecard",
		OCPDevice:      0,
		OscillatorType: "timebeat-rb-ql",
		CardConfig: []string{
			"sma1:out:mac",
			"gnss1:signal:gps+galileo+sbas",
			"osc:type:timebeat-rb-ql",
		},
		Offset:      1000, // nanoseconds
		Atomic:      false,
		MonitorOnly: false,
	}

	handler, err := NewOCPTimecardHandler(config, logger)
	if err != nil {
		t.Fatalf("Failed to create OCP Timecard handler: %v", err)
	}

	if handler == nil {
		t.Fatal("Handler is nil")
	}

	// Проверяем, что обработчик реализует интерфейс
	var _ TimeSourceHandler = handler
}

func TestOCPTimecardHandler_GetConfig(t *testing.T) {
	logger := logrus.New()
	
	config := config.TimeSourceConfig{
		Type:           "ocp_timecard",
		OCPDevice:      0,
		OscillatorType: "timebeat-rb-ql",
		CardConfig: []string{
			"sma1:out:mac",
		},
		Offset: 1000,
	}

	handler, err := NewOCPTimecardHandler(config, logger)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	retrievedConfig := handler.GetConfig()
	if retrievedConfig.Type != "ocp_timecard" {
		t.Errorf("Expected type 'ocp_timecard', got '%s'", retrievedConfig.Type)
	}

	if retrievedConfig.OCPDevice != 0 {
		t.Errorf("Expected OCP device 0, got %d", retrievedConfig.OCPDevice)
	}

	if retrievedConfig.OscillatorType != "timebeat-rb-ql" {
		t.Errorf("Expected oscillator type 'timebeat-rb-ql', got '%s'", retrievedConfig.OscillatorType)
	}

	if retrievedConfig.Offset != 1000 {
		t.Errorf("Expected offset 1000, got %d", retrievedConfig.Offset)
	}
}

func TestOCPTimecardHandler_GetStatus(t *testing.T) {
	logger := logrus.New()
	
	config := config.TimeSourceConfig{
		Type:      "ocp_timecard",
		OCPDevice: 0,
	}

	handler, err := NewOCPTimecardHandler(config, logger)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	status := handler.GetStatus()
	if status.Connected {
		t.Error("Handler should not be connected before Start()")
	}
}

func TestOCPTimecardHandler_StartStop(t *testing.T) {
	logger := logrus.New()
	
	config := config.TimeSourceConfig{
		Type:      "ocp_timecard",
		OCPDevice: 0,
	}

	handler, err := NewOCPTimecardHandler(config, logger)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Попытка запуска без реального устройства должна завершиться ошибкой
	err = handler.Start()
	if err == nil {
		t.Error("Expected error when starting without real device")
	}

	// Stop должен работать даже если Start не был успешным
	err = handler.Stop()
	if err != nil {
		t.Errorf("Stop() should not return error: %v", err)
	}
}

func TestOCPTimecardHandler_GetTimeInfo(t *testing.T) {
	logger := logrus.New()
	
	config := config.TimeSourceConfig{
		Type:      "ocp_timecard",
		OCPDevice: 0,
		Offset:    1000, // nanoseconds
	}

	handler, err := NewOCPTimecardHandler(config, logger)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Без запущенного обработчика GetTimeInfo должен возвращать ошибку
	_, err = handler.GetTimeInfo()
	if err == nil {
		t.Error("Expected error when getting time info without starting handler")
	}
}

func TestOCPTimecardHandler_GetGNSSInfo(t *testing.T) {
	logger := logrus.New()
	
	config := config.TimeSourceConfig{
		Type:      "ocp_timecard",
		OCPDevice: 0,
	}

	handler, err := NewOCPTimecardHandler(config, logger)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	gnssInfo := handler.GetGNSSInfo()
	if gnssInfo.FixType != 0 {
		t.Errorf("Expected initial fix type 0, got %d", gnssInfo.FixType)
	}

	if gnssInfo.SatellitesUsed != 0 {
		t.Errorf("Expected initial satellites used 0, got %d", gnssInfo.SatellitesUsed)
	}
}

func TestOCPTimecardHandler_ConfigureCard(t *testing.T) {
	logger := logrus.New()
	
	config := config.TimeSourceConfig{
		Type:      "ocp_timecard",
		OCPDevice: 0,
		CardConfig: []string{
			"sma1:out:mac",
			"gnss1:signal:gps+galileo+sbas",
			"osc:type:timebeat-rb-ql",
		},
	}

	handler, err := NewOCPTimecardHandler(config, logger)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Приводим к конкретному типу для доступа к приватным методам
	ocpHandler, ok := handler.(*ocpTimecardHandler)
	if !ok {
		t.Fatal("Handler is not of type *ocpTimecardHandler")
	}

	// Тестируем конфигурацию карты (должна завершиться успешно даже без реального устройства)
	err = ocpHandler.configureCard()
	if err != nil {
		t.Errorf("configureCard() should not return error: %v", err)
	}
}

func TestOCPTimecardHandler_DevicePaths(t *testing.T) {
	logger := logrus.New()
	
	config := config.TimeSourceConfig{
		Type:      "ocp_timecard",
		OCPDevice: 1, // тестируем с устройством 1
	}

	handler, err := NewOCPTimecardHandler(config, logger)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	ocpHandler, ok := handler.(*ocpTimecardHandler)
	if !ok {
		t.Fatal("Handler is not of type *ocpTimecardHandler")
	}

	expectedDevicePath := "/sys/class/timecard/ocp1"
	if ocpHandler.devicePath != expectedDevicePath {
		t.Errorf("Expected device path '%s', got '%s'", expectedDevicePath, ocpHandler.devicePath)
	}

	expectedPTPDevice := "/dev/ptp5" // OCP device 1 -> ptp5
	if ocpHandler.ptpDevice != expectedPTPDevice {
		t.Errorf("Expected PTP device '%s', got '%s'", expectedPTPDevice, ocpHandler.ptpDevice)
	}

	expectedGNSSDevice := "/dev/ttyS6" // OCP device 1 -> ttyS6
	if ocpHandler.gnssDevice != expectedGNSSDevice {
		t.Errorf("Expected GNSS device '%s', got '%s'", expectedGNSSDevice, ocpHandler.gnssDevice)
	}
}

func TestOCPTimecardHandler_OffsetCalculation(t *testing.T) {
	logger := logrus.New()
	
	config := config.TimeSourceConfig{
		Type:      "ocp_timecard",
		OCPDevice: 0,
		Offset:    5000, // 5 microseconds
	}

	handler, err := NewOCPTimecardHandler(config, logger)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	ocpHandler, ok := handler.(*ocpTimecardHandler)
	if !ok {
		t.Fatal("Handler is not of type *ocpTimecardHandler")
	}

	// Симулируем данные
	ocpHandler.mu.Lock()
	ocpHandler.lastPPSTime = time.Now().Truncate(time.Second)
	ocpHandler.lastOffset = 100 * time.Nanosecond
	ocpHandler.gnssFixValid = true
	ocpHandler.mu.Unlock()

	timeInfo, err := handler.GetTimeInfo()
	if err != nil {
		t.Fatalf("GetTimeInfo() failed: %v", err)
	}

	// Проверяем, что offset включает статический offset из конфигурации
	expectedOffset := 100*time.Nanosecond + 5000*time.Nanosecond
	if timeInfo.Offset != expectedOffset {
		t.Errorf("Expected offset %v, got %v", expectedOffset, timeInfo.Offset)
	}
}