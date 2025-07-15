package metrics

import (
    "fmt"

    "github.com/sirupsen/logrus"
    "github.com/shiwatime/shiwatime/internal/config"

    beatsclient "github.com/shiwatime/shiwatime/internal/metrics/beats"
)

// ClientInterface определяет методы клиента метрик (native или beats).
type ClientInterface interface {
    Stop() error
    // SendMetric отправляет документ метрики в индекс
    SendMetric(index string, data map[string]interface{})
    // SetupIndexTemplates подготавливает шаблоны индексов при необходимости
    SetupIndexTemplates() error
}

// NewMetricsClient создает клиент метрик в зависимости от config.Output.Type.
// Поддерживаются значения "native" и "beats" (по умолчанию – native).
func NewMetricsClient(outputCfg config.OutputConfig, logger *logrus.Logger) (ClientInterface, error) {
    switch outputCfg.Type {
    case "", "native":
        return NewClient(outputCfg.Elasticsearch, logger)
    case "beats":
        return beatsclient.NewBeatsClient(outputCfg.Beats, logger)
    default:
        return nil, fmt.Errorf("unsupported metrics output type: %s", outputCfg.Type)
    }
}