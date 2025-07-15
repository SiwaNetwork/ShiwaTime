package server

import (
	"fmt"
	"io"
	"time"
	
	"github.com/gliderlabs/ssh"
	"github.com/sirupsen/logrus"
	
	"github.com/shiwatime/shiwatime/internal/clock"
	"github.com/shiwatime/shiwatime/internal/config"
)

// CLIServer SSH CLI сервер
type CLIServer struct {
	config       config.CLIConfig
	clockManager *clock.Manager
	logger       *logrus.Logger
	server       *ssh.Server
}

// NewCLIServer создает новый CLI сервер
func NewCLIServer(cfg config.CLIConfig, clockManager *clock.Manager, logger *logrus.Logger) *CLIServer {
	return &CLIServer{
		config:       cfg,
		clockManager: clockManager,
		logger:       logger,
	}
}

// Start запускает CLI сервер
func (s *CLIServer) Start() error {
	s.server = &ssh.Server{
		Addr: fmt.Sprintf("%s:%d", s.config.BindHost, s.config.BindPort),
		Handler: s.handleSession,
		PasswordHandler: s.handlePassword,
	}
	
	s.logger.WithField("addr", s.server.Addr).Info("Starting CLI server")
	
	return s.server.ListenAndServe()
}

// Stop останавливает CLI сервер
func (s *CLIServer) Stop() error {
	s.logger.Info("Stopping CLI server")
	
	if s.server != nil {
		return s.server.Close()
	}
	
	return nil
}

// handlePassword обрабатывает аутентификацию
func (s *CLIServer) handlePassword(ctx ssh.Context, password string) bool {
	// Простая аутентификация по паролю
	return ctx.User() == s.config.Username && password == s.config.Password
}

// handleSession обрабатывает SSH сессию
func (s *CLIServer) handleSession(sess ssh.Session) {
	user := sess.User()
	s.logger.WithField("user", user).Info("CLI session started")
	
	// Приветствие
	io.WriteString(sess, fmt.Sprintf("Welcome to ShiwaTime CLI\n"))
	io.WriteString(sess, fmt.Sprintf("User: %s\n", user))
	io.WriteString(sess, fmt.Sprintf("Time: %s\n\n", time.Now().Format(time.RFC3339)))
	
	// Простой интерактивный интерфейс
	for {
		io.WriteString(sess, "shiwatime> ")
		
		// Читаем команду (простая реализация)
		buf := make([]byte, 1024)
		n, err := sess.Read(buf)
		if err != nil {
			break
		}
		
		command := string(buf[:n])
		command = command[:len(command)-1] // Убираем \n
		
		if command == "exit" || command == "quit" {
			io.WriteString(sess, "Goodbye!\n")
			break
		}
		
		// Обрабатываем команду
		s.handleCommand(sess, command)
	}
	
	s.logger.WithField("user", user).Info("CLI session ended")
}

// handleCommand обрабатывает команды CLI
func (s *CLIServer) handleCommand(sess ssh.Session, command string) {
	switch command {
	case "status":
		s.handleStatusCommand(sess)
	case "sources":
		s.handleSourcesCommand(sess)
	case "help":
		s.handleHelpCommand(sess)
	case "":
		// Пустая команда, ничего не делаем
	default:
		io.WriteString(sess, fmt.Sprintf("Unknown command: %s\n", command))
		io.WriteString(sess, "Type 'help' for available commands\n")
	}
}

// handleStatusCommand обрабатывает команду status
func (s *CLIServer) handleStatusCommand(sess ssh.Session) {
	state := s.clockManager.GetState()
	selectedSource := s.clockManager.GetSelectedSource()
	
	io.WriteString(sess, fmt.Sprintf("Clock State: %s\n", state.String()))
	
	if selectedSource != nil {
		// Find the name of the selected source
		allSources := s.clockManager.GetSources()
		var sourceName string
		for name, handler := range allSources {
			if handler == selectedSource {
				sourceName = name
				break
			}
		}
		
		config := selectedSource.GetConfig()
		status := selectedSource.GetStatus()
		timeInfo, err := selectedSource.GetTimeInfo()
		
		io.WriteString(sess, fmt.Sprintf("Selected Source: %s (%s)\n", 
			sourceName, config.Type))
		
		if err == nil && timeInfo != nil {
			io.WriteString(sess, fmt.Sprintf("  Offset: %s\n", timeInfo.Offset))
			io.WriteString(sess, fmt.Sprintf("  Quality: %d\n", timeInfo.Quality))
			io.WriteString(sess, fmt.Sprintf("  Last Sync: %s\n", 
				timeInfo.Timestamp.Format(time.RFC3339)))
		} else {
			io.WriteString(sess, "  Status: No time info available\n")
		}
		
		io.WriteString(sess, fmt.Sprintf("  Connected: %v\n", status.Connected))
		if status.LastError != nil {
			io.WriteString(sess, fmt.Sprintf("  Last Error: %s\n", status.LastError.Error()))
		}
	} else {
		io.WriteString(sess, "No source selected\n")
	}
	
	io.WriteString(sess, "\n")
}

// handleSourcesCommand обрабатывает команду sources
func (s *CLIServer) handleSourcesCommand(sess ssh.Session) {
	primarySources, secondarySources := s.clockManager.GetSourcesByPriority()
	selectedSource := s.clockManager.GetSelectedSource()
	
	io.WriteString(sess, "Primary Sources:\n")
	for name, handler := range primarySources {
		status := "inactive"
		handlerStatus := handler.GetStatus()
		config := handler.GetConfig()
		
		if handlerStatus.Connected {
			status = "active"
		}
		if handler == selectedSource {
			status = "selected"
		}
		
		timeInfo, err := handler.GetTimeInfo()
		offset := "unknown"
		quality := 0
		if err == nil && timeInfo != nil {
			offset = timeInfo.Offset.String()
			quality = timeInfo.Quality
		}
		
		io.WriteString(sess, fmt.Sprintf("  %s (%s) - %s\n", 
			name, config.Type, status))
		io.WriteString(sess, fmt.Sprintf("    Offset: %s, Quality: %d\n", 
			offset, quality))
	}
	
	io.WriteString(sess, "\nSecondary Sources:\n")
	for name, handler := range secondarySources {
		status := "inactive"
		handlerStatus := handler.GetStatus()
		config := handler.GetConfig()
		
		if handlerStatus.Connected {
			status = "active"
		}
		if handler == selectedSource {
			status = "selected"
		}
		
		timeInfo, err := handler.GetTimeInfo()
		offset := "unknown"
		quality := 0
		if err == nil && timeInfo != nil {
			offset = timeInfo.Offset.String()
			quality = timeInfo.Quality
		}
		
		io.WriteString(sess, fmt.Sprintf("  %s (%s) - %s\n", 
			name, config.Type, status))
		io.WriteString(sess, fmt.Sprintf("    Offset: %s, Quality: %d\n", 
			offset, quality))
	}
	
	io.WriteString(sess, "\n")
}

// handleHelpCommand обрабатывает команду help
func (s *CLIServer) handleHelpCommand(sess ssh.Session) {
	help := `Available commands:
  status   - Show clock synchronization status
  sources  - Show time sources information
  help     - Show this help message
  exit     - Exit CLI session

`
	io.WriteString(sess, help)
}