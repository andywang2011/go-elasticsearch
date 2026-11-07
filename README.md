# go-elasticsearch

Testable Elasticsearch client for Go with typed `Index[T]`, keyword search, and hybrid kNN + RRF retrieval.

## Install

```bash
go get github.com/andywang2011/go-elasticsearch@v0.1.0
```

## Quick start

```go
client, err := libes.NewClient(libes.Config{
	Addresses: []string{"http://localhost:9200"},
	Username:  "elastic",
	Password:  "secret",
})
if err != nil {
	// handle
}

type Article struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

idx := libes.NewIndex[Article](client, "articles")
result, err := idx.Search(context.Background(), libes.SearchRequest{
	Size: 20,
	Query: libes.BoolQuery(
		[]map[string]any{libes.MultiMatch("keyword", []string{"title^5"}, "and")},
		[]map[string]any{libes.Term("status", 1)},
		nil,
	),
})
_ = result
```

## Examples

```bash
cd go-elasticsearch/examples/search && go run .
```

## License

MIT — see [LICENSE](LICENSE).
