package elasticsearch

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func esServer(h http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		h(w, r)
	}))
}

func newTestIndex(t *testing.T, srv *httptest.Server, cfg Config) IndexAPI[map[string]any] {
	t.Helper()
	cfg.Addresses = []string{srv.URL}
	if cfg.Retry.MaxAttempts == 0 {
		cfg.Retry = RetryConfig{MaxAttempts: 1}
	}
	c, err := NewClientWithTransport(context.Background(), cfg, srv.Client().Transport)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := NewIndex[map[string]any](c, "i")
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

func TestConfigTimeout_appliesToHTTP(t *testing.T) {
	release := make(chan struct{})
	srv := esServer(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	defer srv.Close()
	defer close(release)

	cfg := Config{Addresses: []string{srv.URL}, Timeout: 50 * time.Millisecond, Retry: RetryConfig{MaxAttempts: 1}}
	c, err := NewClientWithTransport(context.Background(), cfg, srv.Client().Transport)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	err = c.Ping(context.Background()) // ctx has no deadline: only Config.Timeout can stop it
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("timeout not enforced, took %v", time.Since(start))
	}
}

func TestConfigTimeout_NewClientDefaultTransport(t *testing.T) {
	srv := esServer(func(w http.ResponseWriter, r *http.Request) { time.Sleep(300 * time.Millisecond) })
	defer srv.Close()
	c, err := NewClient(context.Background(), Config{Addresses: []string{srv.URL}, Timeout: 40 * time.Millisecond, Retry: RetryConfig{MaxAttempts: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Ping(context.Background()); err == nil {
		t.Fatal("expected timeout")
	}
	if tr, ok := c.(*conn); !ok || tr.cfg.Timeout != 40*time.Millisecond {
		t.Fatal("timeout not stored")
	}
}

func TestTimeoutTransport_disabledAndBodyClose(t *testing.T) {
	srv := esServer(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	for _, d := range []time.Duration{0, time.Second} {
		tt := &timeoutTransport{base: http.DefaultTransport, timeout: d}
		res, err := tt.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		if string(b) != "ok" || res.Body.Close() != nil {
			t.Fatal(string(b))
		}
	}
	tt := &timeoutTransport{base: http.DefaultTransport, timeout: time.Second}
	bad, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:1", nil)
	if _, err := tt.RoundTrip(bad); err == nil {
		t.Fatal("expected dial error")
	}
}

func TestNewClient_ctxAndValidation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewClient(ctx, Config{Addresses: []string{"http://x"}}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := NewClientWithTransport(ctx, Config{Addresses: []string{"http://x"}}, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := NewClient(nil, Config{}); err == nil { //nolint:staticcheck
		t.Fatal("expected validation error")
	}
	if _, err := NewClientWithTransport(nil, Config{}, nil); err == nil { //nolint:staticcheck
		t.Fatal("expected validation error")
	}
	if c, err := NewClientWithTransport(context.Background(), Config{Addresses: []string{"http://x"}}, nil); err != nil || c == nil {
		t.Fatal(c, err)
	}
	if _, err := NewClient(context.Background(), Config{Addresses: []string{"http://a b"}}); err == nil {
		t.Fatal("expected es client error for bad address")
	}
}

func TestDefaultBaseTransport_nonHTTPTransport(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	http.DefaultTransport = roundTripperFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("x") })
	if _, ok := defaultBaseTransport(Config{Timeout: time.Second}).(roundTripperFunc); !ok {
		t.Fatal("expected passthrough of non-*http.Transport default")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDefaultBaseTransport_clonesWithDialTimeout(t *testing.T) {
	rt := defaultBaseTransport(Config{Timeout: 3 * time.Second})
	tr, ok := rt.(*http.Transport)
	if !ok || tr == http.DefaultTransport || tr.TLSHandshakeTimeout != 3*time.Second {
		t.Fatalf("%T", rt)
	}
}

func TestTotalHits_parse(t *testing.T) {
	cases := map[string]TotalHits{
		`{"value":42,"relation":"gte"}`: {42, "gte"},
		`{"value":5,"relation":"eq"}`:   {5, "eq"},
		`7`:                             {7, "eq"},
		`null`:                          {},
	}
	for in, want := range cases {
		var got TotalHits
		if err := json.Unmarshal([]byte(in), &got); err != nil || got != want {
			t.Fatalf("%s: %+v %v", in, got, err)
		}
	}
	var bad TotalHits
	if err := json.Unmarshal([]byte(`"x"`), &bad); err == nil {
		t.Fatal("expected error")
	}
	if err := json.Unmarshal([]byte(`{"value":"x"}`), &bad); err == nil {
		t.Fatal("expected error")
	}
}

func TestSearch_totalRelation(t *testing.T) {
	srv := esServer(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":10000,"relation":"gte"},"hits":[{"_source":{"a":1}}]}}`))
	})
	defer srv.Close()
	idx := newTestIndex(t, srv, Config{})
	res, err := idx.Search(context.Background(), SearchRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 10000 || res.TotalRelation != "gte" {
		t.Fatalf("%+v", res)
	}
}

type fakeClient struct{}

func (fakeClient) Ping(context.Context) error { return nil }

func TestNewIndex_unsupportedClient(t *testing.T) {
	if _, err := NewIndex[any](fakeClient{}, "i"); !errors.Is(err, ErrUnsupportedClient) {
		t.Fatal(err)
	}
	if _, err := NewIndex[any](nil, "i"); !errors.Is(err, ErrUnsupportedClient) {
		t.Fatal(err)
	}
	var nilConn *conn
	if _, err := NewIndex[any](nilConn, "i"); !errors.Is(err, ErrUnsupportedClient) {
		t.Fatal(err)
	}
}

func TestSearch_responseTooLarge(t *testing.T) {
	srv := esServer(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hits":{"hits":[]}}`))
	})
	defer srv.Close()
	if _, err := readLimited(strings.NewReader(strings.Repeat("a", 11)), 10); !errors.Is(err, errResponseTooLarge) {
		t.Fatal(err)
	}
	if b, err := readLimited(strings.NewReader("abc"), 10); err != nil || string(b) != "abc" {
		t.Fatal(string(b), err)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestReadHelpers_errors(t *testing.T) {
	if _, err := readLimited(errReader{}, 10); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatal(err)
	}
	if got := readErrorBody(errReader{}); !strings.Contains(got, "unreadable") {
		t.Fatal(got)
	}
	if got := readErrorBody(strings.NewReader(strings.Repeat("x", maxErrorBodyBytes+100))); len(got) != maxErrorBodyBytes {
		t.Fatal(len(got))
	}
}

func TestSearch_truncatedBodyIsError(t *testing.T) {
	// Server advertises more bytes than it sends: ReadAll fails and must surface.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte(`{"hits":`))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
	}))
	defer srv.Close()
	idx := newTestIndex(t, srv, Config{})
	if _, err := idx.Search(context.Background(), SearchRequest{}); err == nil {
		t.Fatal("expected read error for truncated body")
	}
}

func TestSearch_errorBodyAndDecodeError(t *testing.T) {
	status := http.StatusBadRequest
	body := `{"error":"bad"}`
	srv := esServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
	defer srv.Close()
	idx := newTestIndex(t, srv, Config{})
	if _, err := idx.Search(context.Background(), SearchRequest{}); err == nil || !strings.Contains(err.Error(), `"bad"`) {
		t.Fatal(err)
	}
	status, body = http.StatusOK, `not json`
	if _, err := idx.Search(context.Background(), SearchRequest{}); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatal(err)
	}
	status, body = http.StatusBadRequest, `{"error":"idx"}`
	if err := idx.IndexDoc(context.Background(), "1", map[string]any{"a": 1}); err == nil || !strings.Contains(err.Error(), "idx") {
		t.Fatal(err)
	}
	if err := idx.Delete(context.Background(), "1"); err == nil || !strings.Contains(err.Error(), "idx") {
		t.Fatal(err)
	}
	status = http.StatusNotFound
	if err := idx.Delete(context.Background(), "1"); err != nil {
		t.Fatal(err)
	}
}

func TestPing_errorStatus(t *testing.T) {
	srv := esServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("nope"))
	})
	defer srv.Close()
	c, err := NewClientWithTransport(context.Background(), Config{Addresses: []string{srv.URL}, Retry: RetryConfig{MaxAttempts: 1}}, srv.Client().Transport)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Ping(context.Background()); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatal(err)
	}
	var nilC *conn
	if err := nilC.Ping(context.Background()); err == nil {
		t.Fatal("expected not initialised error")
	}
}

func TestIndex_nilClientGuards(t *testing.T) {
	i := &Index[any]{name: "x"}
	if err := i.IndexDoc(context.Background(), "1", 1); err == nil {
		t.Fatal("expected error")
	}
	if err := i.Delete(context.Background(), "1"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := i.Search(context.Background(), SearchRequest{}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := i.HybridSearch(context.Background(), HybridSearchRequest{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestIsRetryableHTTPError_timeoutOnly(t *testing.T) {
	if !isRetryableHTTPError(&net.DNSError{IsTimeout: true}) {
		t.Fatal("timeout must be retryable")
	}
	if isRetryableHTTPError(&net.DNSError{IsTemporary: true}) {
		t.Fatal("temporary-only errors are no longer retried via Temporary()")
	}
	if isRetryableHTTPError(nil) || isRetryableHTTPError(errors.New("plain")) {
		t.Fatal("unexpected retry")
	}
	if isRetryableHTTPError(errors.New("elasticsearch http status abc")) {
		t.Fatal("malformed status must not retry")
	}
}

func TestRetryManager_ctxCancelDuringBackoffAndOp(t *testing.T) {
	rm := newRetryManager(RetryConfig{MaxAttempts: 5, InitialDelay: time.Hour, MaxDelay: time.Hour, Multiplier: 1})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	start := time.Now()
	err := rm.execute(ctx, func(context.Context) error { return httpStatusError(503, "x") })
	if !errors.Is(err, context.Canceled) || time.Since(start) > time.Second {
		t.Fatal(err)
	}

	rm = newRetryManager(DefaultRetryConfig())
	ctx2, cancel2 := context.WithCancel(context.Background())
	err = rm.execute(ctx2, func(context.Context) error { cancel2(); return httpStatusError(503, "x") })
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := rm.execute(nil, func(context.Context) error { return nil }); err != nil { //nolint:staticcheck
		t.Fatal(err)
	}
	if err := rm.sleep(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
}

func TestRetryTransport_unreadableRetryBody(t *testing.T) {
	calls := 0
	base := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{StatusCode: 503, Body: io.NopCloser(errReader{})}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok"))}, nil
	})
	rt := &retryTransport{base: base, retry: newRetryManager(RetryConfig{MaxAttempts: 2, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond, Multiplier: 1})}
	req, _ := http.NewRequest(http.MethodGet, "http://x", nil)
	res, err := rt.RoundTrip(req)
	if err != nil || calls != 2 {
		t.Fatal(err, calls)
	}
	_ = res.Body.Close()
}

func TestRetryTransport_getBodyError(t *testing.T) {
	rt := &retryTransport{base: roundTripperFunc(func(*http.Request) (*http.Response, error) { return nil, nil }),
		retry: newRetryManager(RetryConfig{MaxAttempts: 1})}
	req, _ := http.NewRequest(http.MethodPost, "http://x", strings.NewReader("b"))
	req.GetBody = func() (io.ReadCloser, error) { return nil, errors.New("getbody") }
	if _, err := rt.RoundTrip(req); err == nil {
		t.Fatal("expected GetBody error")
	}
}
