package elasticsearch

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRetryManager_SuccessFirstAttempt(t *testing.T) {
	rm := newRetryManager(RetryConfig{MaxAttempts: 3, InitialDelay: time.Millisecond})
	calls := 0
	err := rm.execute(context.Background(), func(ctx context.Context) error {
		calls++
		return nil
	})
	if err != nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestRetryManager_Retries429(t *testing.T) {
	rm := newRetryManager(RetryConfig{
		MaxAttempts:  3,
		InitialDelay: time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2,
	})
	rm.sleep = func(ctx context.Context, d time.Duration) error { return nil }

	calls := 0
	err := rm.execute(context.Background(), func(ctx context.Context) error {
		calls++
		if calls < 3 {
			return httpStatusError(429, "rate limited")
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestRetryManager_NoRetryOn400(t *testing.T) {
	rm := newRetryManager(RetryConfig{MaxAttempts: 3, InitialDelay: time.Millisecond})
	rm.sleep = func(ctx context.Context, d time.Duration) error {
		t.Fatal("should not sleep on non-retryable error")
		return nil
	}
	calls := 0
	err := rm.execute(context.Background(), func(ctx context.Context) error {
		calls++
		return httpStatusError(400, "bad request")
	})
	if err == nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestIsRetryableHTTPError(t *testing.T) {
	if !isRetryableHTTPError(httpStatusError(503, "unavailable")) {
		t.Error("503 should be retryable")
	}
	if isRetryableHTTPError(httpStatusError(404, "missing")) {
		t.Error("404 should not be retryable")
	}
	if !isRetryableHTTPError(io.EOF) {
		t.Error("EOF should be retryable")
	}
}

func TestRetryTransport_RoundTrip(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("busy"))
			return
		}
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	cfg := Config{Addresses: []string{srv.URL}, Retry: RetryConfig{
		MaxAttempts:  3,
		InitialDelay: time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2,
	}}
	client, err := NewClientWithTransport(context.Background(), cfg, srv.Client().Transport)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping after retry: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestParseHTTPStatusPrefix(t *testing.T) {
	var code int
	n, err := parseHTTPStatusPrefix("elasticsearch http status 429: limit", &code)
	if err != nil || code != 429 || n == 0 {
		t.Fatalf("parse failed: n=%d code=%d err=%v", n, code, err)
	}
}

func TestNormalizeRetry_Defaults(t *testing.T) {
	cfg := RetryConfig{}
	normalizeRetry(&cfg)
	if cfg.MaxAttempts <= 0 {
		t.Error("expected default max attempts")
	}
}

func TestRetryManager_ContextCanceled(t *testing.T) {
	rm := newRetryManager(DefaultRetryConfig())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := rm.execute(ctx, func(ctx context.Context) error {
		return errors.New("should not run")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled, got %v", err)
	}
}

func TestIsRetryableHTTPError_MessagePrefix(t *testing.T) {
	msg := httpStatusError(502, "gateway").Error()
	if !strings.HasPrefix(msg, "elasticsearch http status ") {
		t.Fatal("unexpected message format")
	}
}
