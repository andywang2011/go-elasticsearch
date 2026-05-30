package elasticsearch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"
)

// RetryConfig controls exponential-backoff retry for transient HTTP failures.
type RetryConfig struct {
	// MaxAttempts is the total number of attempts including the first.
	MaxAttempts int
	// InitialDelay is the base delay before the first retry.
	InitialDelay time.Duration
	// MaxDelay caps exponential backoff delay.
	MaxDelay time.Duration
	// Multiplier scales delay after each failed attempt.
	Multiplier float64
}

// DefaultRetryConfig returns HTTP retry defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     2 * time.Second,
		Multiplier:   2.0,
	}
}

type retryManager struct {
	config RetryConfig
	rand   *rand.Rand
	sleep  func(ctx context.Context, d time.Duration) error
}

func newRetryManager(cfg RetryConfig) *retryManager {
	return &retryManager{
		config: cfg,
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())),
		sleep: func(ctx context.Context, d time.Duration) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(d):
				return nil
			}
		},
	}
}

func (rm *retryManager) execute(ctx context.Context, op func(ctx context.Context) error) error {
	var lastErr error
	delay := rm.config.InitialDelay
	for attempt := 0; attempt < rm.config.MaxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := op(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt >= rm.config.MaxAttempts-1 || !isRetryableHTTPError(lastErr) {
			return lastErr
		}
		if err := rm.sleep(ctx, rm.jitter(delay)); err != nil {
			return err
		}
		delay = time.Duration(float64(delay) * rm.config.Multiplier)
		if delay > rm.config.MaxDelay {
			delay = rm.config.MaxDelay
		}
	}
	return lastErr
}

func (rm *retryManager) jitter(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	jitterRange := delay / 5
	if jitterRange == 0 {
		jitterRange = 1
	}
	j := time.Duration(rm.rand.Int63n(int64(jitterRange * 2)))
	return delay - jitterRange + j
}

func normalizeRetry(r *RetryConfig) {
	if r.MaxAttempts <= 0 {
		*r = DefaultRetryConfig()
	}
	if r.InitialDelay <= 0 {
		r.InitialDelay = DefaultRetryConfig().InitialDelay
	}
	if r.MaxDelay <= 0 {
		r.MaxDelay = DefaultRetryConfig().MaxDelay
	}
	if r.Multiplier <= 0 {
		r.Multiplier = DefaultRetryConfig().Multiplier
	}
	if r.MaxDelay < r.InitialDelay {
		r.MaxDelay = r.InitialDelay
	}
}

func isRetryableHTTPError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	msg := err.Error()
	if strings.HasPrefix(msg, "elasticsearch http status ") {
		var code int
		if _, scanErr := parseHTTPStatusPrefix(msg, &code); scanErr == nil && (code == 429 || code >= 500) {
			return true
		}
		return false
	}
	return false
}

func parseHTTPStatusPrefix(msg string, code *int) (int, error) {
	const prefix = "elasticsearch http status "
	if !strings.HasPrefix(msg, prefix) {
		return 0, errors.New("not http status error")
	}
	rest := msg[len(prefix):]
	n := 0
	*code = 0
	for n < len(rest) && rest[n] >= '0' && rest[n] <= '9' {
		*code = *code*10 + int(rest[n]-'0')
		n++
	}
	if n == 0 {
		return 0, errors.New("no status code")
	}
	return n, nil
}

func httpStatusError(code int, body string) error {
	return fmt.Errorf("elasticsearch http status %d: %s", code, body)
}

func isRetryableStatus(code int) bool {
	return code == 429 || code >= 500
}

// retryTransport wraps http.RoundTripper with retry logic.
type retryTransport struct {
	base   http.RoundTripper
	retry  *retryManager
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	err := t.retry.execute(req.Context(), func(ctx context.Context) error {
		r := req
		if req.GetBody != nil {
			cloned := req.Clone(ctx)
			body, bodyErr := req.GetBody()
			if bodyErr != nil {
				return bodyErr
			}
			cloned.Body = body
			r = cloned
		}
		res, doErr := t.base.RoundTrip(r)
		if doErr != nil {
			return doErr
		}
		if isRetryableStatus(res.StatusCode) {
			b, _ := io.ReadAll(res.Body)
			_ = res.Body.Close()
			return httpStatusError(res.StatusCode, string(b))
		}
		resp = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}
