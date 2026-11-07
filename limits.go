package elasticsearch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// maxResponseBytes caps search/ping response bodies read into memory.
	maxResponseBytes = 32 << 20
	// maxErrorBodyBytes caps error bodies embedded in error messages.
	maxErrorBodyBytes = 64 << 10
)

// errResponseTooLarge is returned when a response body exceeds its limit.
var errResponseTooLarge = errors.New("elasticsearch: response body too large")

// readLimited reads at most limit bytes and fails when the body is larger.
func readLimited(r io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: read response body: %w", err)
	}
	if int64(len(b)) > limit {
		return nil, errResponseTooLarge
	}
	return b, nil
}

// readErrorBody returns a bounded, best-effort rendering of an error body.
func readErrorBody(r io.Reader) string {
	b, err := io.ReadAll(io.LimitReader(r, maxErrorBodyBytes))
	if err != nil {
		return fmt.Sprintf("<unreadable body: %v>", err)
	}
	return string(b)
}

// timeoutTransport applies Config.Timeout to every HTTP attempt. The deadline
// stays active until the response body is closed so slow bodies are bounded too.
type timeoutTransport struct {
	base    http.RoundTripper
	timeout time.Duration
}

func (t *timeoutTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.timeout <= 0 {
		return t.base.RoundTrip(req)
	}
	ctx, cancel := context.WithTimeout(req.Context(), t.timeout)
	res, err := t.base.RoundTrip(req.WithContext(ctx))
	if err != nil {
		cancel()
		return nil, err
	}
	res.Body = &cancelBody{ReadCloser: res.Body, cancel: cancel}
	return res, nil
}

type cancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}
