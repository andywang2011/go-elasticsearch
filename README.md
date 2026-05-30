# go-elasticsearch

Testable Elasticsearch client for Go (`github.com/andywang2011/go-elasticsearch`).

## Features

- Typed generic `Index[T]` for index, delete, keyword search, and **hybrid search** (bool query + kNN + RRF)
- Multi-node `Addresses`, configurable refresh policy, HTTP retry
- Config injection in the application layer (no viper inside the library)
- `NewClientWithTransport` for httptest mocks

## Usage

```go
import libes "github.com/andywang2011/go-elasticsearch"

client, err := libes.NewClient(libes.Config{
    Addresses: []string{"http://localhost:9200"},
    Username:  "elastic",
    Password:  "secret",
    Timeout:   10 * time.Second,
})

idx := libes.NewIndex[Article](client, "articles")

// Keyword search
result, err := idx.Search(ctx, libes.SearchRequest{
    Size: 20,
    Query: libes.BoolQuery(
        []map[string]any{libes.MultiMatch("keyword", []string{"title^5"}, "and")},
        []map[string]any{libes.Term("status", 1)},
        nil,
    ),
})

// Hybrid search (keyword + vector + RRF)
hybrid, err := idx.HybridSearch(ctx, libes.HybridSearchRequest{
    Size: 20,
    Query: libes.BoolQuery(must, filter, nil),
    KNN: []libes.KNNQuery{{
        Field: "embedding", QueryVector: vector, K: 10, NumCandidates: 100,
    }},
    Rank: libes.DefaultRRF(),
})
```

## Index mapping (hybrid)

```json
{
  "mappings": {
    "properties": {
      "title": { "type": "text" },
      "embedding": {
        "type": "dense_vector",
        "dims": 1536,
        "index": true,
        "similarity": "cosine"
      }
    }
  }
}
```

Adjust `dims` to match your embedding model. This library does not generate embeddings.

## Testing

```bash
go test ./... -race -count=1
```
