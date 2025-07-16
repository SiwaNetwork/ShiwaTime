package server

import (
	"context"
	"fmt"
	"net/http"
	"time"
	
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	
	"github.com/shiwatime/shiwatime/internal/clock"
	"github.com/shiwatime/shiwatime/internal/config"
	"github.com/shiwatime/shiwatime/internal/protocols"
)

// HTTPServer HTTP сервер для веб-интерфейса
type HTTPServer struct {
	config       config.HTTPConfig
	clockManager *clock.Manager
	logger       *logrus.Logger
	server       *http.Server
}

// StatusResponse ответ статуса
type StatusResponse struct {
	Status         string                 `json:"status"`
	ClockState     string                 `json:"clock_state"`
	SelectedSource *TimeSourceResponse    `json:"selected_source,omitempty"`
	PrimarySources []TimeSourceResponse   `json:"primary_sources"`
	SecondarySources []TimeSourceResponse `json:"secondary_sources"`
	Timestamp      time.Time              `json:"timestamp"`
}

// TimeSourceResponse информация об источнике времени
type TimeSourceResponse struct {
	ID         string    `json:"id"`
	Protocol   string    `json:"protocol"`
	Active     bool      `json:"active"`
	Selected   bool      `json:"selected"`
	LastSync   time.Time `json:"last_sync"`
	Offset     string    `json:"offset"`
	Quality    int       `json:"quality"`
	ErrorCount int       `json:"error_count"`
	LastError  string    `json:"last_error,omitempty"`
}

// NewHTTPServer создает новый HTTP сервер
func NewHTTPServer(cfg config.HTTPConfig, clockManager *clock.Manager, logger *logrus.Logger) *HTTPServer {
	return &HTTPServer{
		config:       cfg,
		clockManager: clockManager,
		logger:       logger,
	}
}

// Start запускает HTTP сервер
func (s *HTTPServer) Start() error {
	// Настраиваем Gin
	if s.logger.Level < logrus.DebugLevel {
		gin.SetMode(gin.ReleaseMode)
	}
	
	router := gin.New()
	router.Use(gin.Recovery())
	
	// Middleware для логирования
	router.Use(gin.LoggerWithWriter(s.logger.Writer()))
	
	// Middleware для CORS
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	})
	
	// Регистрируем маршруты
	s.registerRoutes(router)
	
	// Настраиваем HTTP сервер
	addr := fmt.Sprintf("%s:%d", s.config.BindHost, s.config.BindPort)
	s.server = &http.Server{
		Addr:    addr,
		Handler: router,
	}
	
	s.logger.WithField("addr", addr).Info("Starting HTTP server")
	
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server failed: %w", err)
	}
	
	return nil
}

// Stop останавливает HTTP сервер
func (s *HTTPServer) Stop(ctx context.Context) error {
	s.logger.Info("Stopping HTTP server")
	
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	
	return nil
}

// registerRoutes регистрирует маршруты
func (s *HTTPServer) registerRoutes(router *gin.Engine) {
	// API маршруты
	api := router.Group("/api/v1")
	{
		api.GET("/status", s.handleStatus)
		api.GET("/sources", s.handleSources)
		api.GET("/sources/:id", s.handleSourceDetails)
		api.GET("/health", s.handleHealth)
	}
	
	// Статические файлы и UI
	router.GET("/", s.handleIndex)
	router.GET("/ui/*filepath", s.handleUI)
	
	// Метрики в формате Prometheus (опционально)
	router.GET("/metrics", s.handleMetrics)
}

// handleStatus обрабатывает запрос статуса
func (s *HTTPServer) handleStatus(c *gin.Context) {
	primarySources, secondarySources := s.clockManager.GetSourcesByPriority()
	selectedSource := s.clockManager.GetSelectedSource()
	
	response := StatusResponse{
		Status:           "ok",
		ClockState:       s.clockManager.GetState().String(),
		PrimarySources:   convertSources(primarySources),
		SecondarySources: convertSources(secondarySources),
		Timestamp:        time.Now(),
	}
	
	if selectedSource != nil {
		// Find the name of the selected source
		allSources := s.clockManager.GetSources()
		for name, handler := range allSources {
			if handler == selectedSource {
				sourceResp := convertSource(name, handler)
				sourceResp.Selected = true
				response.SelectedSource = &sourceResp
				break
			}
		}
	}
	
	c.JSON(http.StatusOK, response)
}

// handleSources обрабатывает запрос списка источников
func (s *HTTPServer) handleSources(c *gin.Context) {
	primarySources, secondarySources := s.clockManager.GetSourcesByPriority()
	
	response := map[string]interface{}{
		"primary_sources":   convertSources(primarySources),
		"secondary_sources": convertSources(secondarySources),
		"timestamp":         time.Now(),
	}
	
	c.JSON(http.StatusOK, response)
}

// handleSourceDetails обрабатывает запрос деталей источника
func (s *HTTPServer) handleSourceDetails(c *gin.Context) {
	sourceID := c.Param("id")
	
	allSources := s.clockManager.GetSources()
	
	for name, source := range allSources {
		if name == sourceID {
			response := map[string]interface{}{
				"source": convertSource(name, source),
				"timestamp": time.Now(),
			}
			
			c.JSON(http.StatusOK, response)
			return
		}
	}
	
	c.JSON(http.StatusNotFound, gin.H{"error": "Source not found"})
}

// handleHealth обрабатывает запрос здоровья
func (s *HTTPServer) handleHealth(c *gin.Context) {
	state := s.clockManager.GetState()
	
	status := "healthy"
	if state == clock.ClockStateUnsynchronized || state == clock.ClockStateUnknown {
		status = "unhealthy"
	}
	
	response := map[string]interface{}{
		"status":      status,
		"clock_state": state.String(),
		"timestamp":   time.Now(),
	}
	
	c.JSON(http.StatusOK, response)
}

// handleIndex обрабатывает главную страницу
func (s *HTTPServer) handleIndex(c *gin.Context) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>ShiwaTime - Time Synchronization</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .header { background: #f8f9fa; padding: 20px; border-radius: 5px; margin-bottom: 20px; }
        .status { margin: 20px 0; }
        .source { background: #fff; border: 1px solid #ddd; padding: 15px; margin: 10px 0; border-radius: 5px; }
        .active { border-left: 4px solid #28a745; }
        .selected { border-left: 4px solid #007bff; }
        .inactive { border-left: 4px solid #dc3545; }
        .refresh { margin: 20px 0; }
    </style>
    <script>
        async function loadStatus() {
            try {
                const response = await fetch('/api/v1/status');
                const data = await response.json();
                document.getElementById('status').innerHTML = renderStatus(data);
            } catch (error) {
                document.getElementById('status').innerHTML = '<p>Error loading status: ' + error.message + '</p>';
            }
        }
        
        function renderStatus(data) {
            let html = '<h2>Clock Status: ' + data.clock_state + '</h2>';
            
            if (data.selected_source) {
                html += '<p><strong>Selected Source:</strong> ' + data.selected_source.id + ' (' + data.selected_source.protocol + ')</p>';
            }
            
            html += '<h3>Primary Sources</h3>';
            html += renderSources(data.primary_sources);
            
            html += '<h3>Secondary Sources</h3>';
            html += renderSources(data.secondary_sources);
            
            html += '<p><small>Last updated: ' + new Date(data.timestamp).toLocaleString() + '</small></p>';
            
            return html;
        }
        
        function renderSources(sources) {
            if (!sources || sources.length === 0) {
                return '<p>No sources configured</p>';
            }
            
            let html = '';
            sources.forEach(source => {
                let className = source.active ? 'source active' : 'source inactive';
                if (source.selected) className = 'source selected';
                
                html += '<div class="' + className + '">';
                html += '<h4>' + source.id + ' (' + source.protocol + ')</h4>';
                html += '<p>Status: ' + (source.active ? 'Active' : 'Inactive') + '</p>';
                html += '<p>Offset: ' + source.offset + '</p>';
                html += '<p>Quality: ' + source.quality + '</p>';
                if (source.last_error) {
                    html += '<p style="color: red;">Error: ' + source.last_error + '</p>';
                }
                html += '</div>';
            });
            
            return html;
        }
        
        // Автообновление каждые 5 секунд
        setInterval(loadStatus, 5000);
        window.onload = loadStatus;
    </script>
</head>
<body>
    <div class="header">
        <h1>ShiwaTime - Time Synchronization Software</h1>
        <p>Real-time monitoring of time synchronization sources</p>
    </div>
    
    <div class="refresh">
        <button onclick="loadStatus()">Refresh</button>
    </div>
    
    <div id="status">Loading...</div>
    
    <div style="margin-top: 40px; padding-top: 20px; border-top: 1px solid #ddd;">
        <h3>API Endpoints</h3>
        <ul>
            <li><a href="/api/v1/status">/api/v1/status</a> - System status</li>
            <li><a href="/api/v1/sources">/api/v1/sources</a> - Time sources</li>
            <li><a href="/api/v1/health">/api/v1/health</a> - Health check</li>
            <li><a href="/metrics">/metrics</a> - Prometheus metrics</li>
        </ul>
    </div>
</body>
</html>`
	
	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, html)
}

// handleUI обрабатывает UI файлы
func (s *HTTPServer) handleUI(c *gin.Context) {
	// Пока просто редирект на главную
	c.Redirect(http.StatusTemporaryRedirect, "/")
}

// handleMetrics обрабатывает запрос метрик
func (s *HTTPServer) handleMetrics(c *gin.Context) {
	// Простые метрики в формате Prometheus
	allSources := s.clockManager.GetSources()
	
	metrics := fmt.Sprintf("# HELP shiwatime_clock_state Current clock state (0=unknown, 1=synchronized, 2=unsynchronized)\n")
	metrics += fmt.Sprintf("# TYPE shiwatime_clock_state gauge\n")
	metrics += fmt.Sprintf("shiwatime_clock_state %d\n", int(s.clockManager.GetState()))
	
	metrics += fmt.Sprintf("# HELP shiwatime_sources_total Total number of time sources\n")
	metrics += fmt.Sprintf("# TYPE shiwatime_sources_total gauge\n")
	metrics += fmt.Sprintf("shiwatime_sources_total %d\n", len(allSources))
	
	metrics += fmt.Sprintf("# HELP shiwatime_sources_active Number of active time sources\n")
	metrics += fmt.Sprintf("# TYPE shiwatime_sources_active gauge\n")
	activeCount := 0
	for _, source := range allSources {
		status := source.GetStatus()
		if status.Connected {
			activeCount++
		}
	}
	metrics += fmt.Sprintf("shiwatime_sources_active %d\n", activeCount)
	
	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, metrics)
}

// convertSources конвертирует источники в ответ
func convertSources(sources map[string]protocols.TimeSourceHandler) []TimeSourceResponse {
	result := make([]TimeSourceResponse, 0, len(sources))
	for name, handler := range sources {
		result = append(result, convertSource(name, handler))
	}
	return result
}

// convertSource конвертирует источник в ответ
func convertSource(name string, handler protocols.TimeSourceHandler) TimeSourceResponse {
	status := handler.GetStatus()
	config := handler.GetConfig()
	
	timeInfo, err := handler.GetTimeInfo()
	offset := "unknown"
	quality := 0
	lastSync := time.Time{}
	
	if err == nil && timeInfo != nil {
		offset = timeInfo.Offset.String()
		quality = timeInfo.Quality
		lastSync = timeInfo.Timestamp
	}
	
	resp := TimeSourceResponse{
		ID:         name,
		Protocol:   config.Type,
		Active:     status.Connected,
		Selected:   false, // Will be set separately for selected source
		LastSync:   lastSync,
		Offset:     offset,
		Quality:    quality,
		ErrorCount: int(status.ErrorCount),
	}
	
	if status.LastError != nil {
		resp.LastError = status.LastError.Error()
	}
	
	return resp
}

// convertDurations конвертирует длительности в строки
func convertDurations(durations []time.Duration) []string {
	result := make([]string, len(durations))
	for i, d := range durations {
		result[i] = d.String()
	}
	return result
}