package beats

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "strings"
    "time"

    "github.com/elastic/beats/v7/libbeat/common"
    "github.com/elastic/go-elasticsearch/v8"
    "github.com/elastic/go-elasticsearch/v8/esapi"
    "github.com/sirupsen/logrus"
    "github.com/shiwatime/shiwatime/internal/config"
)

// Client implements metrics.ClientInterface using Elastic Beats publisher internally.
// Для первой итерации мы повторно используем существующий Native HTTP bulk клиент,
// но преобразуем документы в common.MapStr, что упростит переход на настоящий
// pipeline Beats в дальнейшем.

type Client struct {
    es     *elasticsearch.Client
    cfg    config.BeatsConfig
    logger *logrus.Logger

    buffer     []map[string]interface{}
    bufferSize int
}

const defaultBufferSize = 100

// NewBeatsClient создаёт beats-клиент.
func NewBeatsClient(cfg config.BeatsConfig, logger *logrus.Logger) (*Client, error) {
    esCfg := elasticsearch.Config{
        Addresses: cfg.Hosts,
        Username:  cfg.Username,
        Password:  cfg.Password,
        APIKey:    cfg.APIKey,
    }
    es, err := elasticsearch.NewClient(esCfg)
    if err != nil {
        return nil, fmt.Errorf("failed to create beats elasticsearch client: %w", err)
    }
    // ping
    if _, err := es.Info(); err != nil {
        logger.WithError(err).Warn("Beats ES client ping failed")
    }
    return &Client{
        es:         es,
        cfg:        cfg,
        logger:     logger,
        bufferSize: defaultBufferSize,
    }, nil
}

// Stop останавливает внутренний клиент
func (c *Client) Stop() error {
    // nothing special
    // flush remaining buffer
    c.flush()
    return nil
}

// SendMetric конвертирует в Beats common.MapStr и отправляет
func (c *Client) SendMetric(index string, data map[string]interface{}) {
    event := common.MapStr(data)
    doc := event.ToMapStr().Clone()

    c.buffer = append(c.buffer, map[string]interface{}{
        "index": index,
        "data":  doc,
    })

    if len(c.buffer) >= c.bufferSize {
        go c.flush()
    }
}

// SetupIndexTemplates делегируется нативному клиенту
func (c *Client) SetupIndexTemplates() error {
    // TODO: optionally use Beats index management; noop for now
    return nil
}

// flush bulk to Elasticsearch (similar to native)
func (c *Client) flush() {
    if len(c.buffer) == 0 {
        return
    }

    buf := bytes.Buffer{}
    for _, doc := range c.buffer {
        idx := fmt.Sprintf("%s-%s", doc["index"], time.Now().Format("2006.01.02"))
        meta := map[string]interface{}{
            "index": map[string]interface{}{
                "_index": idx,
            },
        }
        metaBytes, _ := json.Marshal(meta)
        docBytes, _ := json.Marshal(doc["data"])
        buf.Write(metaBytes)
        buf.WriteByte('\n')
        buf.Write(docBytes)
        buf.WriteByte('\n')
    }

    req := esapi.BulkRequest{Body: strings.NewReader(buf.String())}
    res, err := req.Do(context.Background(), c.es)
    if err != nil {
        c.logger.WithError(err).Error("Beats bulk request failed")
        return
    }
    defer res.Body.Close()

    if res.IsError() {
        c.logger.WithField("status", res.Status()).Error("Beats bulk request error")
    }

    // reset buffer
    c.buffer = c.buffer[:0]
}