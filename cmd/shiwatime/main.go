package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	
	"github.com/shiwatime/shiwatime/internal/clock"
	"github.com/shiwatime/shiwatime/internal/config"
	"github.com/shiwatime/shiwatime/internal/metrics"
	"github.com/shiwatime/shiwatime/internal/server"
)

var (
	configPath string
	logLevel   string
	version    = "1.0.0"
	buildTime  = "unknown"
	gitCommit  = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "shiwatime",
		Short: "ShiwaTime - Time Synchronization Software",
		Long: `ShiwaTime — это приложение для синхронизации времени, написанное на Go, 
которое повторяет функционал Timebeat. Поддерживает множество протоколов 
синхронизации времени и предоставляет мониторинг через Elasticsearch.`,
		Run: runShiwaTime,
	}
	
	// Глобальные флаги
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "config/shiwatime.yml", "Path to config file")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
	
	// Команда version
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("ShiwaTime %s\n", version)
			fmt.Printf("Build time: %s\n", buildTime)
			fmt.Printf("Git commit: %s\n", gitCommit)
		},
	}
	
	// Команда config
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management",
	}
	
	validateConfigCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		Run:   validateConfig,
	}
	
	showConfigCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		Run:   showConfig,
	}
	
	configCmd.AddCommand(validateConfigCmd, showConfigCmd)
	rootCmd.AddCommand(versionCmd, configCmd)
	
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runShiwaTime(cmd *cobra.Command, args []string) {
	// Настраиваем логгер
	logger := logrus.New()
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logger.Fatal("Invalid log level: ", logLevel)
	}
	logger.SetLevel(level)
	
	// Настраиваем формат логов
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		TimestampFormat: time.RFC3339,
	})
	
	logger.WithFields(logrus.Fields{
		"version":    version,
		"build_time": buildTime,
		"git_commit": gitCommit,
	}).Info("Starting ShiwaTime")
	
	// Загружаем конфигурацию
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logger.Fatal("Failed to load config: ", err)
	}
	
	logger.WithField("config_path", configPath).Info("Loaded configuration")
	
	// Создаем клиент метрик
	var metricsClient *metrics.Client
	if len(cfg.Output.Elasticsearch.Hosts) > 0 {
		metricsClient, err = metrics.NewClient(cfg.Output.Elasticsearch, logger)
		if err != nil {
			logger.WithError(err).Error("Failed to create metrics client, continuing without metrics")
		} else {
			// Настраиваем шаблоны индексов
			if err := metricsClient.SetupIndexTemplates(); err != nil {
				logger.WithError(err).Warn("Failed to setup index templates")
			}
		}
	}
	
	// Создаем менеджер часов
	clockManager, err := clock.NewManager(cfg, logger, metricsClient)
	if err != nil {
		logger.Fatal("Failed to create clock manager: ", err)
	}
	
	// Создаем HTTP сервер если включен
	var httpServer *server.HTTPServer
	if cfg.ShiwaTime.HTTP.Enable {
		httpServer = server.NewHTTPServer(cfg.ShiwaTime.HTTP, clockManager, logger)
	}
	
	// Создаем SSH CLI сервер если включен
	var cliServer *server.CLIServer
	if cfg.ShiwaTime.CLI.Enable {
		cliServer = server.NewCLIServer(cfg.ShiwaTime.CLI, clockManager, logger)
	}
	
	// Запускаем все сервисы
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Запускаем менеджер часов
	if err := clockManager.Start(); err != nil {
		logger.Fatal("Failed to start clock manager: ", err)
	}
	
	// Запускаем HTTP сервер
	if httpServer != nil {
		go func() {
			if err := httpServer.Start(); err != nil {
				logger.WithError(err).Error("HTTP server failed")
			}
		}()
	}
	
	// Запускаем CLI сервер
	if cliServer != nil {
		go func() {
			if err := cliServer.Start(); err != nil {
				logger.WithError(err).Error("CLI server failed")
			}
		}()
	}
	
	logger.Info("ShiwaTime started successfully")
	
	// Ждем сигнал завершения
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	
	select {
	case sig := <-sigCh:
		logger.WithField("signal", sig).Info("Received shutdown signal")
	case <-ctx.Done():
		logger.Info("Context cancelled")
	}
	
	// Graceful shutdown
	logger.Info("Shutting down ShiwaTime...")
	
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	// Останавливаем сервисы
	if cliServer != nil {
		cliServer.Stop()
	}
	
	if httpServer != nil {
		httpServer.Stop(shutdownCtx)
	}
	
	if err := clockManager.Stop(); err != nil {
		logger.WithError(err).Error("Failed to stop clock manager")
	}
	
	if metricsClient != nil {
		if err := metricsClient.Stop(); err != nil {
			logger.WithError(err).Error("Failed to stop metrics client")
		}
	}
	
	logger.Info("ShiwaTime stopped")
}

func validateConfig(cmd *cobra.Command, args []string) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	_, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration validation failed: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("Configuration is valid")
}

func showConfig(cmd *cobra.Command, args []string) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Отключаем логи
	
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}
	
	// Можно добавить вывод конфигурации в YAML формате
	fmt.Printf("Configuration loaded from: %s\n", configPath)
	fmt.Printf("Primary time sources: %d\n", len(cfg.ShiwaTime.ClockSync.PrimaryClocks))
	fmt.Printf("Secondary time sources: %d\n", len(cfg.ShiwaTime.ClockSync.SecondaryClocks))
	fmt.Printf("Clock adjustment enabled: %t\n", cfg.ShiwaTime.ClockSync.AdjustClock)
	fmt.Printf("Step limit: %s\n", cfg.ShiwaTime.ClockSync.StepLimit)
	
	if cfg.ShiwaTime.HTTP.Enable {
		fmt.Printf("HTTP server: enabled on %s:%d\n", cfg.ShiwaTime.HTTP.BindHost, cfg.ShiwaTime.HTTP.BindPort)
	} else {
		fmt.Printf("HTTP server: disabled\n")
	}
	
	if cfg.ShiwaTime.CLI.Enable {
		fmt.Printf("CLI server: enabled on %s:%d\n", cfg.ShiwaTime.CLI.BindHost, cfg.ShiwaTime.CLI.BindPort)
	} else {
		fmt.Printf("CLI server: disabled\n")
	}
	
	fmt.Printf("Elasticsearch hosts: %v\n", cfg.Output.Elasticsearch.Hosts)
}