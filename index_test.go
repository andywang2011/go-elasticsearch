package elasticsearch_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	libes "github.com/andywang2011/go-elasticsearch"
)

func fakeESServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		handler(w, r)
	}))
}

func TestClient_Search(t *testing.T) {
	resp := []byte(`{"hits":{"total":{"value":2},"hits":[
		{"_source":{"id":1,"title":"a1"},"sort":[1.5,"2025-01-01"]},
		{"_source":{"id":2,"title":"a2"},"sort":[2.5,"2025-01-02"]}
	]}}`)
	srv := fakeESServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/test-index/_search" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(resp)
	})
	defer srv.Close()

	cfg := libes.Config{Addresses: []string{srv.URL}}
	client, err := libes.NewClientWithTransport(context.Background(), cfg, srv.Client().Transport)
	if err != nil {
		t.Fatalf("NewClientWithTransport: %v", err)
	}

	type Doc struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	}

	idx, err := libes.NewIndex[Doc](client, "test-index")
	if err != nil {
		t.Fatal(err)
	}
	result, err := idx.Search(context.Background(), libes.SearchRequest{
		Size:  10,
		Query: libes.Term("status", 1),
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if result.Total != 2 || len(result.List) != 2 {
		t.Errorf("expected total=2 list=2, got total=%d list=%d", result.Total, len(result.List))
	}
	if len(result.Next) != 2 {
		t.Errorf("expected next len=2, got %d", len(result.Next))
	}
}

func TestClient_HybridSearch(t *testing.T) {
	resp := []byte(`{"hits":{"total":{"value":1},"hits":[{"_source":{"id":1,"title":"hybrid"}}]}}`)
	var gotBody string
	srv := fakeESServer(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(resp)
	})
	defer srv.Close()

	cfg := libes.Config{Addresses: []string{srv.URL}}
	client, err := libes.NewClientWithTransport(context.Background(), cfg, srv.Client().Transport)
	if err != nil {
		t.Fatal(err)
	}

	type Doc struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	}
	idx, err := libes.NewIndex[Doc](client, "articles")
	if err != nil {
		t.Fatal(err)
	}
	_, err = idx.HybridSearch(context.Background(), libes.HybridSearchRequest{
		Size: 10,
		Query: libes.BoolQuery(
			[]map[string]any{libes.MultiMatch("hello", []string{"title"}, "and")},
			nil, nil,
		),
		KNN: []libes.KNNQuery{{
			Field:         "embedding",
			QueryVector:   []float32{0.1, 0.2, 0.3},
			K:             5,
			NumCandidates: 50,
		}},
		Rank: libes.DefaultRRF(),
	})
	if err != nil {
		t.Fatalf("HybridSearch: %v", err)
	}
	if !strings.Contains(gotBody, `"knn"`) || !strings.Contains(gotBody, `"rank"`) {
		t.Fatalf("expected knn and rank in body, got %s", gotBody)
	}
}

func TestClient_IndexDocAndDelete(t *testing.T) {
	srv := fakeESServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"created"}`))
	})
	defer srv.Close()

	cfg := libes.Config{Addresses: []string{srv.URL}}
	client, err := libes.NewClientWithTransport(context.Background(), cfg, srv.Client().Transport)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := libes.NewIndex[map[string]any](client, "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.IndexDoc(context.Background(), "1", map[string]any{"id": 1}); err != nil {
		t.Fatalf("IndexDoc: %v", err)
	}
	if err := idx.Delete(context.Background(), "1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestClient_Ping(t *testing.T) {
	srv := fakeESServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	cfg := libes.Config{Addresses: []string{srv.URL}}
	client, err := libes.NewClientWithTransport(context.Background(), cfg, srv.Client().Transport)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}
