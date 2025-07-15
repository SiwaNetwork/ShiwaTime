package metrics

import "time"

// Event represents a generic metrics event for publishing via either the native ES client or libbeat publisher.
type Event struct {
    Timestamp time.Time              `json:"@timestamp"`
    Fields    map[string]interface{} `json:"fields"`
}