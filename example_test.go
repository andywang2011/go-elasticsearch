package elasticsearch_test

import (
	"context"

	libes "github.com/andywang2011/go-elasticsearch"
)

func ExampleNewIndex_search() {
	client, err := libes.NewClient(context.Background(), libes.Config{
		Addresses: []string{"http://localhost:9200"},
		Username:  "elastic",
		Password:  "secret",
	})
	if err != nil {
		return
	}

	type Article struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	}

	idx, err := libes.NewIndex[Article](client, "articles")
	if err != nil {
		return
	}
	result, err := idx.Search(context.Background(), libes.SearchRequest{
		Size: 20,
		Query: libes.BoolQuery(
			[]map[string]any{libes.MultiMatch("keyword", []string{"title^5"}, "and")},
			[]map[string]any{libes.Term("status", 1)},
			nil,
		),
	})
	if err != nil {
		return
	}
	_ = result
}
