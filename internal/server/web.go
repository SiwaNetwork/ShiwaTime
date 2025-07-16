package server

import (
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/shiwatime/shiwatime/internal/clock"
	"github.com/shiwatime/shiwatime/internal/protocols"
)

// WebData структура данных для веб-интерфейса
type WebData struct {
	CurrentTime     string
	ClockState      string
	ClockStatus     string
	LastSync        string
	StabilityLevel  string
	StabilityPercent int
	
	ActiveSources   int
	TotalSources    int
	Sources         []SourceInfo
	
	Statistics      StatisticsInfo
}

// SourceInfo информация об источнике времени для веб-интерфейса
type SourceInfo struct {
	Type          string
	Host          string
	Interface     string
	Device        string
	Status        string
	StatusText    string
	Offset        string
	Delay         string
	Quality       string
	
	// PTP specific
	PTPDomain     int
	PTPPortState  string
	PTPMaster     *PTPMasterInfo
	
	// PPS specific
	PPSMode       string
	PPSEventCount uint64
	PPSLastEvent  *PPSEventInfo
	
	// PHC specific
	PHCIndex      int
	PHCMaxAdj     int64
	PHCPPSAvail   bool
}

// PTPMasterInfo информация о PTP мастере
type PTPMasterInfo struct {
	ClockIdentity string
	ClockClass    int
}

// PPSEventInfo информация о PPS событии
type PPSEventInfo struct {
	Timestamp time.Time
}

// StatisticsInfo расширенная статистика
type StatisticsInfo struct {
	MeanOffset     time.Duration
	MeanJitter     time.Duration
	AllanDeviation float64
	Correlation    float64
	FreqOffset     float64
	KernelSync     bool
	Stable         bool
	MaxOffset      time.Duration
	MinOffset      time.Duration
}

// webTemplate HTML template для веб-интерфейса
const webTemplate = `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ShiwaTime - Мониторинг точного времени</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: #333;
            min-height: 100vh;
        }
        
        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 20px;
        }
        
        .header {
            text-align: center;
            color: white;
            margin-bottom: 30px;
        }
        
        .header h1 {
            font-size: 2.5em;
            margin-bottom: 10px;
            text-shadow: 2px 2px 4px rgba(0,0,0,0.3);
        }
        
        .header .subtitle {
            font-size: 1.2em;
            opacity: 0.9;
        }
        
        .grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
            gap: 20px;
            margin-bottom: 20px;
        }
        
        .card {
            background: rgba(255, 255, 255, 0.95);
            border-radius: 15px;
            padding: 25px;
            box-shadow: 0 8px 32px rgba(0,0,0,0.1);
            backdrop-filter: blur(10px);
            border: 1px solid rgba(255,255,255,0.2);
            transition: transform 0.3s ease, box-shadow 0.3s ease;
        }
        
        .card:hover {
            transform: translateY(-5px);
            box-shadow: 0 12px 40px rgba(0,0,0,0.15);
        }
        
        .card h2 {
            color: #4a5568;
            margin-bottom: 15px;
            font-size: 1.4em;
            border-bottom: 2px solid #e2e8f0;
            padding-bottom: 8px;
        }
        
        .status-indicator {
            display: inline-block;
            width: 12px;
            height: 12px;
            border-radius: 50%;
            margin-right: 8px;
        }
        
        .status-good { background-color: #48bb78; }
        .status-warning { background-color: #ed8936; }
        .status-error { background-color: #f56565; }
        .status-unknown { background-color: #a0aec0; }
        
        .metric {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 8px 0;
            border-bottom: 1px solid #f1f5f9;
        }
        
        .metric:last-child {
            border-bottom: none;
        }
        
        .metric-label {
            font-weight: 500;
            color: #4a5568;
        }
        
        .metric-value {
            font-weight: 600;
            color: #2d3748;
            font-family: 'Courier New', monospace;
        }
        
        .time-display {
            text-align: center;
            font-size: 2em;
            font-weight: bold;
            color: #2b6cb0;
            margin: 15px 0;
            text-shadow: 1px 1px 2px rgba(0,0,0,0.1);
        }
        
        .sources-grid {
            display: grid;
            gap: 15px;
            margin-top: 15px;
        }
        
        .source-card {
            background: #f8fafc;
            border-radius: 10px;
            padding: 15px;
            border-left: 4px solid #4299e1;
        }
        
        .source-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 10px;
        }
        
        .source-type {
            font-weight: bold;
            color: #2b6cb0;
            text-transform: uppercase;
            font-size: 0.9em;
        }
        
        .protocol-badge {
            padding: 4px 8px;
            border-radius: 20px;
            font-size: 0.8em;
            font-weight: bold;
            text-transform: uppercase;
        }
        
        .protocol-ntp { background-color: #bee3f8; color: #2b6cb0; }
        .protocol-ptp { background-color: #c6f6d5; color: #22543d; }
        .protocol-pps { background-color: #fed7d7; color: #742a2a; }
        .protocol-phc { background-color: #faf5ff; color: #553c9a; }
        .protocol-nmea { background-color: #feebcb; color: #744210; }
        
        .advanced-stats {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 10px;
            margin-top: 15px;
        }
        
        .stat-box {
            background: #f0f9ff;
            border: 1px solid #e0f2fe;
            border-radius: 8px;
            padding: 12px;
            text-align: center;
        }
        
        .stat-value {
            font-size: 1.2em;
            font-weight: bold;
            color: #0369a1;
        }
        
        .stat-label {
            font-size: 0.8em;
            color: #64748b;
            margin-top: 4px;
        }
        
        .protocol-details {
            background: #f0fdf4;
            border: 1px solid #dcfce7;
            border-radius: 8px;
            padding: 15px;
            margin-top: 10px;
        }
        
        .stability-indicator {
            display: flex;
            align-items: center;
            margin: 10px 0;
        }
        
        .stability-bar {
            flex-grow: 1;
            height: 8px;
            background: #e2e8f0;
            border-radius: 4px;
            margin: 0 10px;
            overflow: hidden;
        }
        
        .stability-fill {
            height: 100%;
            transition: width 0.3s ease;
        }
        
        .stability-good { background: linear-gradient(to right, #48bb78, #38a169); }
        .stability-warning { background: linear-gradient(to right, #ed8936, #dd6b20); }
        .stability-bad { background: linear-gradient(to right, #f56565, #e53e3e); }
        
        .auto-refresh {
            display: flex;
            align-items: center;
            justify-content: center;
            margin-top: 20px;
            color: white;
        }
        
        .auto-refresh input {
            margin-right: 10px;
        }
        
        @media (max-width: 768px) {
            .grid {
                grid-template-columns: 1fr;
            }
            
            .advanced-stats {
                grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
            }
            
            .time-display {
                font-size: 1.5em;
            }
            
            .header h1 {
                font-size: 2em;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🕐 ShiwaTime</h1>
            <div class="subtitle">Система синхронизации точного времени</div>
        </div>
        
        <div class="grid">
            <!-- Системное время -->
            <div class="card">
                <h2>🕒 Системное время</h2>
                <div class="time-display" id="current-time">{{.CurrentTime}}</div>
                <div class="metric">
                    <span class="metric-label">Состояние часов</span>
                    <span class="metric-value">
                        <span class="status-indicator status-{{.ClockStatus}}"></span>
                        {{.ClockState}}
                    </span>
                </div>
                <div class="metric">
                    <span class="metric-label">Последняя синхронизация</span>
                    <span class="metric-value">{{.LastSync}}</span>
                </div>
                <div class="stability-indicator">
                    <span class="metric-label">Стабильность</span>
                    <div class="stability-bar">
                        <div class="stability-fill stability-{{.StabilityLevel}}" style="width: {{.StabilityPercent}}%"></div>
                    </div>
                    <span class="metric-value">{{.StabilityPercent}}%</span>
                </div>
            </div>
            
            <!-- Источники времени -->
            <div class="card">
                <h2>📡 Источники времени</h2>
                <div class="metric">
                    <span class="metric-label">Активных источников</span>
                    <span class="metric-value">{{.ActiveSources}} / {{.TotalSources}}</span>
                </div>
                <div class="sources-grid">
                    {{range .Sources}}
                    <div class="source-card">
                        <div class="source-header">
                            <span class="source-type">{{.Type}}</span>
                            <span class="protocol-badge protocol-{{.Type}}">{{.Type}}</span>
                        </div>
                        <div class="metric">
                            <span class="metric-label">Статус</span>
                            <span class="metric-value">
                                <span class="status-indicator status-{{.Status}}"></span>
                                {{.StatusText}}
                            </span>
                        </div>
                        {{if .Host}}
                        <div class="metric">
                            <span class="metric-label">Хост</span>
                            <span class="metric-value">{{.Host}}</span>
                        </div>
                        {{end}}
                        {{if .Interface}}
                        <div class="metric">
                            <span class="metric-label">Интерфейс</span>
                            <span class="metric-value">{{.Interface}}</span>
                        </div>
                        {{end}}
                        {{if .Device}}
                        <div class="metric">
                            <span class="metric-label">Устройство</span>
                            <span class="metric-value">{{.Device}}</span>
                        </div>
                        {{end}}
                        <div class="metric">
                            <span class="metric-label">Смещение</span>
                            <span class="metric-value">{{.Offset}}</span>
                        </div>
                        <div class="metric">
                            <span class="metric-label">Задержка</span>
                            <span class="metric-value">{{.Delay}}</span>
                        </div>
                        <div class="metric">
                            <span class="metric-label">Качество</span>
                            <span class="metric-value">{{.Quality}}</span>
                        </div>
                        
                        <!-- Протокол-специфичная информация -->
                        {{if eq .Type "ptp"}}
                        <div class="protocol-details">
                            <div class="metric">
                                <span class="metric-label">PTP Домен</span>
                                <span class="metric-value">{{.PTPDomain}}</span>
                            </div>
                            <div class="metric">
                                <span class="metric-label">Состояние порта</span>
                                <span class="metric-value">{{.PTPPortState}}</span>
                            </div>
                            {{if .PTPMaster}}
                            <div class="metric">
                                <span class="metric-label">Master ID</span>
                                <span class="metric-value">{{.PTPMaster.ClockIdentity}}</span>
                            </div>
                            <div class="metric">
                                <span class="metric-label">Clock Class</span>
                                <span class="metric-value">{{.PTPMaster.ClockClass}}</span>
                            </div>
                            {{end}}
                        </div>
                        {{else if eq .Type "pps"}}
                        <div class="protocol-details">
                            <div class="metric">
                                <span class="metric-label">Режим сигнала</span>
                                <span class="metric-value">{{.PPSMode}}</span>
                            </div>
                            <div class="metric">
                                <span class="metric-label">Событий</span>
                                <span class="metric-value">{{.PPSEventCount}}</span>
                            </div>
                            {{if .PPSLastEvent}}
                            <div class="metric">
                                <span class="metric-label">Последнее событие</span>
                                <span class="metric-value">{{.PPSLastEvent.Timestamp.Format "15:04:05"}}</span>
                            </div>
                            {{end}}
                        </div>
                        {{else if eq .Type "phc"}}
                        <div class="protocol-details">
                            <div class="metric">
                                <span class="metric-label">PHC Индекс</span>
                                <span class="metric-value">{{.PHCIndex}}</span>
                            </div>
                            <div class="metric">
                                <span class="metric-label">Max Adj</span>
                                <span class="metric-value">{{.PHCMaxAdj}} ppb</span>
                            </div>
                            <div class="metric">
                                <span class="metric-label">PPS доступен</span>
                                <span class="metric-value">{{if .PHCPPSAvail}}Да{{else}}Нет{{end}}</span>
                            </div>
                        </div>
                        {{end}}
                    </div>
                    {{end}}
                </div>
            </div>
            
            <!-- Расширенная статистика -->
            <div class="card">
                <h2>📊 Расширенная статистика</h2>
                <div class="advanced-stats">
                    <div class="stat-box">
                        <div class="stat-value">{{.Statistics.MeanOffset}}</div>
                        <div class="stat-label">Среднее смещение</div>
                    </div>
                                         <div class="stat-box">
                         <div class="stat-value">{{.Statistics.MaxOffset}}</div>
                         <div class="stat-label">Максимальное смещение</div>
                     </div>
                    <div class="stat-box">
                        <div class="stat-value">{{.Statistics.MeanJitter}}</div>
                        <div class="stat-label">Средний джиттер</div>
                    </div>
                    <div class="stat-box">
                        <div class="stat-value">{{printf "%.2e" .Statistics.AllanDeviation}}</div>
                        <div class="stat-label">Allan Deviation</div>
                    </div>
                    <div class="stat-box">
                        <div class="stat-value">{{printf "%.3f" .Statistics.Correlation}}</div>
                        <div class="stat-label">Корреляция</div>
                    </div>
                    <div class="stat-box">
                        <div class="stat-value">{{printf "%.1f" .Statistics.FreqOffset}}</div>
                        <div class="stat-label">Частотное смещение (ppb)</div>
                    </div>
                </div>
                
                <div class="metric">
                    <span class="metric-label">Ядерная синхронизация</span>
                    <span class="metric-value">{{if .Statistics.KernelSync}}Включена{{else}}Отключена{{end}}</span>
                </div>
                <div class="metric">
                    <span class="metric-label">Стабильность часов</span>
                    <span class="metric-value">
                        <span class="status-indicator status-{{if .Statistics.Stable}}good{{else}}warning{{end}}"></span>
                        {{if .Statistics.Stable}}Стабильные{{else}}Нестабильные{{end}}
                    </span>
                </div>
                <div class="metric">
                    <span class="metric-label">Диапазон смещений</span>
                    <span class="metric-value">{{.Statistics.MinOffset}} - {{.Statistics.MaxOffset}}</span>
                </div>
            </div>
        </div>
        
        <div class="auto-refresh">
            <input type="checkbox" id="auto-refresh" checked>
            <label for="auto-refresh">Автообновление (10 сек)</label>
        </div>
    </div>
    
    <script>
        // Автообновление страницы
        let autoRefreshInterval;
        
        function toggleAutoRefresh() {
            const checkbox = document.getElementById('auto-refresh');
            if (checkbox.checked) {
                autoRefreshInterval = setInterval(() => {
                    location.reload();
                }, 10000);
            } else {
                clearInterval(autoRefreshInterval);
            }
        }
        
        // Обновление времени в реальном времени
        function updateTime() {
            const timeElement = document.getElementById('current-time');
            if (timeElement) {
                const now = new Date();
                timeElement.textContent = now.toLocaleTimeString('ru-RU', {
                    hour12: false,
                    hour: '2-digit',
                    minute: '2-digit',
                    second: '2-digit'
                });
            }
        }
        
        // Инициализация
        document.addEventListener('DOMContentLoaded', function() {
            toggleAutoRefresh();
            setInterval(updateTime, 1000);
            updateTime();
            
            document.getElementById('auto-refresh').addEventListener('change', toggleAutoRefresh);
        });
    </script>
</body>
</html>
`

// WebHandler обрабатывает веб-интерфейс
type WebHandler struct {
	template     *template.Template
	clockManager *clock.Manager
}

// NewWebHandler создает новый веб-обработчик
func NewWebHandler(clockManager *clock.Manager) (*WebHandler, error) {
	tmpl, err := template.New("web").Parse(webTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse web template: %w", err)
	}

	return &WebHandler{
		template:     tmpl,
		clockManager: clockManager,
	}, nil
}

// ServeHTTP обрабатывает HTTP запросы
func (h *WebHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data := h.buildWebData()
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := h.template.Execute(w, data)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// buildWebData собирает данные для веб-интерфейса
func (h *WebHandler) buildWebData() WebData {
	now := time.Now()
	
	// Получаем статистику часов
	stats := h.clockManager.GetStatistics()
	
	// Получаем источники времени
	sources := h.buildSourcesInfo()
	
	// Подсчитываем активные источники
	activeSources := 0
	for _, source := range sources {
		if source.Status == "good" {
			activeSources++
		}
	}
	
	// Определяем уровень стабильности
	stabilityLevel := "bad"
	stabilityPercent := 0
	if stats.Stable {
		stabilityLevel = "good"
		stabilityPercent = 90
	} else if stats.AllanDeviation < 1e-6 {
		stabilityLevel = "warning"
		stabilityPercent = 60
	} else {
		stabilityPercent = 30
	}
	
	return WebData{
		CurrentTime:      now.Format("15:04:05"),
		ClockState:       stats.State.String(),
		ClockStatus:      h.getClockStatusClass(stats.State),
		LastSync:         "недавно", // TODO: получить реальное время последней синхронизации
		StabilityLevel:   stabilityLevel,
		StabilityPercent: stabilityPercent,
		
		ActiveSources: activeSources,
		TotalSources:  len(sources),
		Sources:       sources,
		
		Statistics: StatisticsInfo{
			MeanOffset:     stats.MeanOffset,
			MeanJitter:     stats.MeanJitter,
			AllanDeviation: stats.AllanDeviation,
			Correlation:    0, // stats.Correlation not available
			FreqOffset:     stats.FreqOffset,
			KernelSync:     stats.KernelSync,
			Stable:         stats.Stable,
			MaxOffset:      stats.MaxOffset,
			MinOffset:      stats.MinOffset,
		},
	}
}

// buildSourcesInfo собирает информацию об источниках времени
func (h *WebHandler) buildSourcesInfo() []SourceInfo {
	sources := h.clockManager.GetSources()
	var sourceInfos []SourceInfo
	
	for _, handler := range sources {
		config := handler.GetConfig()
		status := handler.GetStatus()
		
		sourceInfo := SourceInfo{
			Type:       config.Type,
			Host:       config.Host,
			Interface:  config.Interface,
			Device:     config.Device,
			Status:     h.getSourceStatusClass(status),
			StatusText: h.getSourceStatusText(status),
		}
		
		// Получаем информацию о времени
		timeInfo, err := handler.GetTimeInfo()
		if err == nil {
			sourceInfo.Offset = timeInfo.Offset.String()
			sourceInfo.Delay = timeInfo.Delay.String()
			sourceInfo.Quality = fmt.Sprintf("%d", timeInfo.Quality)
		} else {
			sourceInfo.Offset = "N/A"
			sourceInfo.Delay = "N/A"
			sourceInfo.Quality = "N/A"
		}
		
		// Добавляем протокол-специфичную информацию
		h.addProtocolSpecificInfo(&sourceInfo, handler)
		
		sourceInfos = append(sourceInfos, sourceInfo)
	}
	
	return sourceInfos
}

// addProtocolSpecificInfo добавляет протокол-специфичную информацию
func (h *WebHandler) addProtocolSpecificInfo(info *SourceInfo, handler protocols.TimeSourceHandler) {
	switch info.Type {
	case "ptp":
		// Пытаемся получить PTP-специфичную информацию
		if ptpHandler, ok := handler.(protocols.PTPHandler); ok {
			info.PTPDomain = ptpHandler.GetDomain()
			info.PTPPortState = ptpHandler.GetPortState().String()
			
			if masterInfo := ptpHandler.GetMasterInfo(); masterInfo != nil {
				info.PTPMaster = &PTPMasterInfo{
					ClockIdentity: masterInfo.ClockIdentity,
					ClockClass:    masterInfo.ClockClass,
				}
			}
		}
		
	case "pps":
		// PPS информация
		if ppsHandler, ok := handler.(protocols.PPSHandler); ok {
			info.PPSEventCount = ppsHandler.GetPulseCount()
			
			lastTime := ppsHandler.GetLastPulseTime()
			if !lastTime.IsZero() {
				info.PPSLastEvent = &PPSEventInfo{
					Timestamp: lastTime,
				}
			}
		}
		
	case "phc":
		// PHC информация - интерфейс PHCHandler не определен, пропускаем специфичную информацию
		// TODO: Добавить PHCHandler interface для получения PHC-специфичной информации
	}
}

// getClockStatusClass возвращает CSS класс для состояния часов
func (h *WebHandler) getClockStatusClass(state clock.ClockState) string {
	switch state {
	case clock.ClockStateSynchronized:
		return "good"
	case clock.ClockStateSynchronizing:
		return "warning"
	case clock.ClockStateHoldover:
		return "warning"
	case clock.ClockStateUnsynchronized:
		return "error"
	default:
		return "unknown"
	}
}

// getSourceStatusClass возвращает CSS класс для статуса источника
func (h *WebHandler) getSourceStatusClass(status protocols.ConnectionStatus) string {
	if !status.Connected {
		return "error"
	}
	
	// Проверяем актуальность активности
	if time.Since(status.LastActivity) > 30*time.Second {
		return "warning"
	}
	
	if status.LastError != nil {
		return "warning"
	}
	
	return "good"
}

// getSourceStatusText возвращает текстовое описание статуса источника
func (h *WebHandler) getSourceStatusText(status protocols.ConnectionStatus) string {
	if !status.Connected {
		return "Отключен"
	}
	
	if status.LastError != nil {
		return "Ошибка"
	}
	
	if time.Since(status.LastActivity) > 30*time.Second {
		return "Неактивен"
	}
	
	return "Активен"
}