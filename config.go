package elasticsearch

import (
	"fmt"
	"time"
)

const defaultTimeout = 10 * time.Second

// RefreshPolicy controls index/delete refresh behavior.
type RefreshPolicy string

const (
	// RefreshFalse does not force refresh (default, best throughput).
	RefreshFalse RefreshPolicy = "false"
	// RefreshWaitFor waits until the next refresh interval.
	RefreshWaitFor RefreshPolicy = "wait_for"
	// RefreshTrue forces immediate refresh (use sparingly).
	RefreshTrue RefreshPolicy = "true"
)

// Config holds Elasticsearch client settings.
// Fill this struct in the application layer (not from viper inside this library).
type Config struct {
	// Addresses lists Elasticsearch node URLs for failover.
	Addresses []string
	// Username is the basic auth username when set.
	Username string
	// Password is the basic auth password when set.
	Password string
	// Timeout is the HTTP client timeout.
	Timeout time.Duration
	// Retry controls transient HTTP retry behavior.
	Retry RetryConfig
	// RefreshPolicy applies to index and delete operations.
	RefreshPolicy RefreshPolicy
}

// DefaultConfig returns defaults for tests and local development.
func DefaultConfig() Config {
	return Config{
		Timeout:       defaultTimeout,
		Retry:         DefaultRetryConfig(),
		RefreshPolicy: RefreshFalse,
	}
}

// ConfigDefault is an alias for DefaultConfig (naming consistency across go-* libs).
func ConfigDefault() Config { return DefaultConfig() }

// Validate checks required fields and applies defaults.
func (c *Config) Validate() error {
	if len(c.Addresses) == 0 {
		return fmt.Errorf("elasticsearch config: at least one address is required")
	}
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}
	if c.Retry.MaxAttempts <= 0 {
		c.Retry = DefaultRetryConfig()
	} else {
		normalizeRetry(&c.Retry)
	}
	if c.RefreshPolicy == "" {
		c.RefreshPolicy = RefreshFalse
	}
	switch c.RefreshPolicy {
	case RefreshFalse, RefreshWaitFor, RefreshTrue:
	default:
		return fmt.Errorf("elasticsearch config: invalid refresh policy %q", c.RefreshPolicy)
	}
	return nil
}
