package elasticsearch

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
)

type conn struct {
	cfg    Config
	client *es.Client
}

var _ Client = (*conn)(nil)

// ErrUnsupportedClient is returned when a Client was not created by this package.
var ErrUnsupportedClient = errors.New("elasticsearch: unsupported client implementation")

// defaultBaseTransport returns a private clone of http.DefaultTransport whose
// dial timeout follows cfg.Timeout.
func defaultBaseTransport(cfg Config) http.RoundTripper {
	if dt, ok := http.DefaultTransport.(*http.Transport); ok {
		t := dt.Clone()
		t.DialContext = (&net.Dialer{Timeout: cfg.Timeout, KeepAlive: 30 * time.Second}).DialContext
		t.TLSHandshakeTimeout = cfg.Timeout
		return t
	}
	return http.DefaultTransport
}

func buildTransport(cfg Config, base http.RoundTripper) http.RoundTripper {
	return &retryTransport{
		base:  &timeoutTransport{base: base, timeout: cfg.Timeout},
		retry: newRetryManager(cfg.Retry),
	}
}

// NewClient creates an Elasticsearch client from Config. Config.Timeout bounds
// every HTTP attempt. ctx is checked before construction (the client itself
// connects lazily).
func NewClient(ctx context.Context, cfg Config) (Client, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return newClientWithRoundTripper(cfg, buildTransport(cfg, defaultBaseTransport(cfg)))
}

// NewClientWithTransport injects an HTTP RoundTripper for tests.
func NewClientWithTransport(ctx context.Context, cfg Config, rt http.RoundTripper) (Client, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if rt == nil {
		rt = defaultBaseTransport(cfg)
	}
	return newClientWithRoundTripper(cfg, buildTransport(cfg, rt))
}

func ctxErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func newClientWithRoundTripper(cfg Config, rt http.RoundTripper) (*conn, error) {
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
	return &conn{cfg: cfg, client: esClient}, nil
}

// Ping checks cluster availability.
func (c *conn) Ping(ctx context.Context) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("elasticsearch: client is not initialized")
	}
	resp, err := c.client.Ping(c.client.Ping.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return fmt.Errorf("elasticsearch: ping: %s: %s", resp.Status(), readErrorBody(resp.Body))
	}
	return nil
}

func (c *conn) refresh() string {
	return string(c.cfg.RefreshPolicy)
}
