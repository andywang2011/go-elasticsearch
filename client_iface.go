package elasticsearch

import "context"

// Client is the Elasticsearch connection surface (mock-friendly).
type Client interface {
	Ping(ctx context.Context) error
}
