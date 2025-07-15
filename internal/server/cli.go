package server

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	
	ssh "github.com/gliderlabs/ssh"
	"github.com/sirupsen/logrus"
	
	cryptossh "golang.org/x/crypto/ssh"
	
	"github.com/shiwatime/shiwatime/internal/clock"
	"github.com/shiwatime/shiwatime/internal/config"
)

// CLIServer SSH CLI сервер
type CLIServer struct {
	config       config.CLIConfig
	clockManager *clock.Manager
	logger       *logrus.Logger
	server       *ssh.Server

	mu             sync.Mutex
	activeSessions int
	authorizedKeys map[string]cryptossh.PublicKey
}

// NewCLIServer создает новый CLI сервер
func NewCLIServer(cfg config.CLIConfig, clockManager *clock.Manager, logger *logrus.Logger) *CLIServer {
	return &CLIServer{
		config:       cfg,
		clockManager: clockManager,
		logger:       logger,
		authorizedKeys: map[string]cryptossh.PublicKey{},
	}
}

// Start запускает CLI сервер
func (s *CLIServer) Start() error {
	// Загружаем authorized_keys, если указан путь
	if s.config.AuthorizedKeys != "" {
		if err := s.loadAuthorizedKeys(s.config.AuthorizedKeys); err != nil {
			s.logger.WithError(err).Warn("Failed to load authorized_keys file")
		}
	}

	s.server = &ssh.Server{
		Addr: fmt.Sprintf("%s:%d", s.config.BindHost, s.config.BindPort),
		Handler: s.handleSession,
		PasswordHandler: s.handlePassword,
		PublicKeyHandler: s.handlePublicKey,
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
	if ctx.User() != s.config.Username || password != s.config.Password {
		return false
	}
	// Проверка лимита сессий
	return s.incrementSession()
}

// handlePublicKey проверяет ключ в authorized_keys и лимит сессий
func (s *CLIServer) handlePublicKey(ctx ssh.Context, key ssh.PublicKey) bool {
	fingerprint := cryptossh.FingerprintSHA256(key)
	if _, ok := s.authorizedKeys[fingerprint]; !ok {
		return false
	}
	return s.incrementSession()
}

// incrementSession увеличивает счетчик и проверяет лимит
func (s *CLIServer) incrementSession() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.config.MaxSessions > 0 && s.activeSessions >= s.config.MaxSessions {
		return false
	}
	s.activeSessions++
	return true
}

func (s *CLIServer) decrementSession() {
	s.mu.Lock()
	if s.activeSessions > 0 {
		s.activeSessions--
	}
	s.mu.Unlock()
}

// handleSession обрабатывает SSH сессию
func (s *CLIServer) handleSession(sess ssh.Session) {
	user := sess.User()
	s.logger.WithField("user", user).Info("CLI session started")

	// Ensure counter is decremented on exit
	defer s.decrementSession()
	
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
	case "sessions":
		s.handleSessionsCommand(sess)
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
		io.WriteString(sess, fmt.Sprintf("Selected Source: %s (%s)\n", 
			selectedSource.ID, selectedSource.Protocol))
		io.WriteString(sess, fmt.Sprintf("  Offset: %s\n", selectedSource.Status.Offset))
		io.WriteString(sess, fmt.Sprintf("  Quality: %d\n", selectedSource.Status.Quality))
		io.WriteString(sess, fmt.Sprintf("  Last Sync: %s\n", 
			selectedSource.Status.LastSync.Format(time.RFC3339)))
	} else {
		io.WriteString(sess, "No source selected\n")
	}
	
	io.WriteString(sess, "\n")
}

// handleSourcesCommand обрабатывает команду sources
func (s *CLIServer) handleSourcesCommand(sess ssh.Session) {
	primarySources, secondarySources := s.clockManager.GetSources()
	
	io.WriteString(sess, "Primary Sources:\n")
	for _, source := range primarySources {
		status := "inactive"
		if source.Status.Active {
			status = "active"
		}
		if source.Status.Selected {
			status = "selected"
		}
		
		io.WriteString(sess, fmt.Sprintf("  %s (%s) - %s\n", 
			source.ID, source.Protocol, status))
		io.WriteString(sess, fmt.Sprintf("    Offset: %s, Quality: %d\n", 
			source.Status.Offset, source.Status.Quality))
	}
	
	io.WriteString(sess, "\nSecondary Sources:\n")
	for _, source := range secondarySources {
		status := "inactive"
		if source.Status.Active {
			status = "active"
		}
		if source.Status.Selected {
			status = "selected"
		}
		
		io.WriteString(sess, fmt.Sprintf("  %s (%s) - %s\n", 
			source.ID, source.Protocol, status))
		io.WriteString(sess, fmt.Sprintf("    Offset: %s, Quality: %d\n", 
			source.Status.Offset, source.Status.Quality))
	}
	
	io.WriteString(sess, "\n")
}

// handleSessionsCommand выводит количество активных сессий и лимит
func (s *CLIServer) handleSessionsCommand(sess ssh.Session) {
	s.mu.Lock()
	current := s.activeSessions
	max := s.config.MaxSessions
	s.mu.Unlock()

	io.WriteString(sess, fmt.Sprintf("Active sessions: %d / %d\n\n", current, max))
}

// handleHelpCommand обрабатывает команду help
func (s *CLIServer) handleHelpCommand(sess ssh.Session) {
	help := `Available commands:
  sessions - Show active SSH session count
  status   - Show clock synchronization status
  sources  - Show time sources information
  help     - Show this help message
  exit     - Exit CLI session

`
	io.WriteString(sess, help)
}

// loadAuthorizedKeys читает файл authorized_keys и заполняет карту отпечатков
func (s *CLIServer) loadAuthorizedKeys(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pubKey, _, _, _, err := cryptossh.ParseAuthorizedKey([]byte(line))
		if err != nil {
			s.logger.WithError(err).Warn("Skipping invalid public key entry")
			continue
		}
		fp := cryptossh.FingerprintSHA256(pubKey)
		s.authorizedKeys[fp] = pubKey
	}
	return scanner.Err()
}