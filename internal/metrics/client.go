package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/sirupsen/logrus"
	"github.com/shiwatime/shiwatime/internal/config"
)

// Client клиент для работы с Elasticsearch
type Client struct {
	es     *elasticsearch.Client
	config config.ElasticsearchConfig
	logger *logrus.Logger
	
	buffer     []MetricDocument
	bufferMu   sync.Mutex
	bufferSize int
	
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// MetricDocument представляет документ метрики
type MetricDocument struct {
	Index string
	Data  map[string]interface{}
}

// NewClient создает новый клиент метрик
func NewClient(cfg config.ElasticsearchConfig, logger *logrus.Logger) (*Client, error) {
	// Настраиваем конфигурацию Elasticsearch
	esCfg := elasticsearch.Config{
		Addresses: cfg.Hosts,
	}
	
	// Добавляем аутентификацию если настроена
	if cfg.Username != "" && cfg.Password != "" {
		esCfg.Username = cfg.Username
		esCfg.Password = cfg.Password
	}
	
	if cfg.APIKey != "" {
		esCfg.APIKey = cfg.APIKey
	}
	
	// Создаем клиент
	es, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch client: %w", err)
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	client := &Client{
		es:         es,
		config:     cfg,
		logger:     logger,
		bufferSize: 100, // по умолчанию
		ctx:        ctx,
		cancel:     cancel,
	}
	
	// Проверяем соединение
	if err := client.ping(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to ping elasticsearch: %w", err)
	}
	
	// Запускаем фоновую отправку метрик
	client.wg.Add(1)
	go client.flushLoop()
	
	return client, nil
}

// Stop останавливает клиент
func (c *Client) Stop() error {
	c.logger.Info("Stopping metrics client")
	
	c.cancel()
	
	// Отправляем оставшиеся метрики
	c.flush()
	
	c.wg.Wait()
	
	return nil
}

// SendMetric отправляет метрику
func (c *Client) SendMetric(index string, data map[string]interface{}) {
	doc := MetricDocument{
		Index: index,
		Data:  data,
	}
	
	c.bufferMu.Lock()
	c.buffer = append(c.buffer, doc)
	shouldFlush := len(c.buffer) >= c.bufferSize
	c.bufferMu.Unlock()
	
	if shouldFlush {
		go c.flush()
	}
}

// ping проверяет соединение с Elasticsearch
func (c *Client) ping() error {
	res, err := c.es.Info()
	if err != nil {
		return err
	}
	defer res.Body.Close()
	
	if res.IsError() {
		return fmt.Errorf("elasticsearch returned error: %s", res.Status())
	}
	
	c.logger.Info("Successfully connected to Elasticsearch")
	return nil
}

// flushLoop периодически отправляет накопленные метрики
func (c *Client) flushLoop() {
	defer c.wg.Done()
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.flush()
		}
	}
}

// flush отправляет накопленные метрики
func (c *Client) flush() {
	c.bufferMu.Lock()
	if len(c.buffer) == 0 {
		c.bufferMu.Unlock()
		return
	}
	
	docs := make([]MetricDocument, len(c.buffer))
	copy(docs, c.buffer)
	c.buffer = c.buffer[:0]
	c.bufferMu.Unlock()
	
	if err := c.sendBatch(docs); err != nil {
		c.logger.WithError(err).Error("Failed to send metrics batch")
		
		// Возвращаем документы в буфер при ошибке
		c.bufferMu.Lock()
		c.buffer = append(docs, c.buffer...)
		c.bufferMu.Unlock()
	}
}

// sendBatch отправляет пакет документов
func (c *Client) sendBatch(docs []MetricDocument) error {
	if len(docs) == 0 {
		return nil
	}
	
	var buf bytes.Buffer
	
	for _, doc := range docs {
		// Генерируем индекс с текущей датой
		index := fmt.Sprintf("%s-%s", doc.Index, time.Now().Format("2006.01.02"))
		
		// Метаданные для bulk API
		meta := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": index,
			},
		}
		
		metaData, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		
		docData, err := json.Marshal(doc.Data)
		if err != nil {
			return fmt.Errorf("failed to marshal document: %w", err)
		}
		
		buf.Write(metaData)
		buf.WriteByte('\n')
		buf.Write(docData)
		buf.WriteByte('\n')
	}
	
	// Отправляем через bulk API
	req := esapi.BulkRequest{
		Body: strings.NewReader(buf.String()),
	}
	
	res, err := req.Do(c.ctx, c.es)
	if err != nil {
		return fmt.Errorf("bulk request failed: %w", err)
	}
	defer res.Body.Close()
	
	if res.IsError() {
		return fmt.Errorf("bulk request returned error: %s", res.Status())
	}
	
	// Парсим ответ для проверки ошибок
	var response struct {
		Errors bool `json:"errors"`
		Items  []map[string]interface{} `json:"items"`
	}
	
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode bulk response: %w", err)
	}
	
	if response.Errors {
		c.logger.Warn("Some documents in bulk request failed")
		// Можно добавить более детальную обработку ошибок
	}
	
	c.logger.WithField("count", len(docs)).Debug("Successfully sent metrics batch")
	
	return nil
}

// CreateIndexTemplate создает шаблон индекса для метрик
func (c *Client) CreateIndexTemplate(name string, pattern string, mappings map[string]interface{}) error {
	template := map[string]interface{}{
		"index_patterns": []string{pattern},
		"template": map[string]interface{}{
			"settings": map[string]interface{}{
				"number_of_shards":   1,
				"number_of_replicas": 0,
				"index": map[string]interface{}{
					"refresh_interval": "5s",
				},
			},
			"mappings": mappings,
		},
	}
	
	data, err := json.Marshal(template)
	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}
	
	req := esapi.IndicesPutIndexTemplateRequest{
		Name: name,
		Body: bytes.NewReader(data),
	}
	
	res, err := req.Do(c.ctx, c.es)
	if err != nil {
		return fmt.Errorf("failed to create index template: %w", err)
	}
	defer res.Body.Close()
	
	if res.IsError() {
		return fmt.Errorf("index template creation returned error: %s", res.Status())
	}
	
	c.logger.WithFields(logrus.Fields{
		"template": name,
		"pattern":  pattern,
	}).Info("Created index template")
	
	return nil
}

// SetupIndexTemplates создает необходимые шаблоны индексов
func (c *Client) SetupIndexTemplates() error {
	// Шаблон для метрик часов
	clockMappings := map[string]interface{}{
		"properties": map[string]interface{}{
			"@timestamp": map[string]interface{}{
				"type": "date",
			},
			"clock_state": map[string]interface{}{
				"type": "keyword",
			},
			"selected_source": map[string]interface{}{
				"type": "keyword",
			},
		},
	}
	
	if err := c.CreateIndexTemplate("shiwatime-clock", "shiwatime_clock-*", clockMappings); err != nil {
		return err
	}
	
	// Шаблон для метрик источников времени
	sourceMappings := map[string]interface{}{
		"properties": map[string]interface{}{
			"@timestamp": map[string]interface{}{
				"type": "date",
			},
			"source_id": map[string]interface{}{
				"type": "keyword",
			},
			"protocol": map[string]interface{}{
				"type": "keyword",
			},
			"active": map[string]interface{}{
				"type": "boolean",
			},
			"selected": map[string]interface{}{
				"type": "boolean",
			},
			"offset_ns": map[string]interface{}{
				"type": "long",
			},
			"quality": map[string]interface{}{
				"type": "integer",
			},
			"error_count": map[string]interface{}{
				"type": "integer",
			},
			"packets_received": map[string]interface{}{
				"type": "long",
			},
			"sync_count": map[string]interface{}{
				"type": "long",
			},
		},
	}
	
	if err := c.CreateIndexTemplate("shiwatime-source", "shiwatime_source-*", sourceMappings); err != nil {
		return err
	}
	
	return nil
}