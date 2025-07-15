package metrics

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "log"
    "time"

    elastic "github.com/elastic/go-elasticsearch/v8"
)

type Client struct {
    es *elastic.Client
    index string
}

type Event struct {
    Timestamp time.Time              `json:"@timestamp"`
    Fields    map[string]interface{} `json:"fields"`
}

func New(hosts []string, index string) (*Client, error) {
    cfg := elastic.Config{Addresses: hosts}
    es, err := elastic.NewClient(cfg)
    if err != nil {
        return nil, err
    }
    return &Client{es: es, index: index}, nil
}

func (c *Client) Publish(ctx context.Context, evt Event) error {
    body, err := json.Marshal(evt)
    if err != nil {
        return err
    }
    res, err := c.es.Index(c.index, bytes.NewReader(body))
    if err != nil {
        return err
    }
    defer res.Body.Close()
    if res.IsError() {
        return fmt.Errorf("elasticsearch index error: %s", res.String())
    }
    return nil
}

func (c *Client) PublishAsync(ctx context.Context, evt Event) {
    go func() {
        if err := c.Publish(ctx, evt); err != nil {
            log.Printf("[metrics] publish error: %v", err)
        }
    }()
}