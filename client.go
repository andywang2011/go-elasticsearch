package elasticsearch

import (
	"context"
	"fmt"
	"net/http"

	es "github.com/elastic/go-elasticsearch/v8"
)

// API is the Elasticsearch client surface.
type API interface {
	Ping(ctx context.Context) error
}

// Client implements API and backs generic Index operations.
type Client struct {
	cfg    Config
	client *es.Client
}

// NewClient creates an Elasticsearch client from Config.
func NewClient(cfg Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	base := http.DefaultTransport
	if base == nil {
		base = &http.Transport{}
	}
	rt := &retryTransport{base: base, retry: newRetryManager(cfg.Retry)}
	return newClientWithRoundTripper(cfg, rt)
}

// NewClientWithTransport injects an HTTP RoundTripper for tests.
func NewClientWithTransport(cfg Config, rt http.RoundTripper) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if rt == nil {
		rt = http.DefaultTransport
	}
	return newClientWithRoundTripper(cfg, &retryTransport{base: rt, retry: newRetryManager(cfg.Retry)})
}

func newClientWithRoundTripper(cfg Config, rt http.RoundTripper) (*Client, error) {
	esCfg := es.Config{
		Addresses: cfg.Addresses,
		Username:  cfg.Username,
		Password:  cfg.Password,
		Transport: rt,
	}
	esClient, err := es.NewClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: connect: %w", err)
	}
	return &Client{cfg: cfg, client: esClient}, nil
}

// Ping checks cluster availability.
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("elasticsearch: client is not initialized")
	}
	resp, err := c.client.Ping(c.client.Ping.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return fmt.Errorf("elasticsearch: ping: %s", resp.String())
	}
	return nil
}

func (c *Client) refresh() string {
	return string(c.cfg.RefreshPolicy)
}
