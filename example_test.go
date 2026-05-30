package elasticsearch_test

import (
	"context"
	"fmt"
	"log"

	libes "github.com/andywang2011/go-elasticsearch"
)

func ExampleNewIndex_search() {
	client, err := libes.NewClient(libes.Config{
		Addresses: []string{"http://localhost:9200"},
		Username:  "elastic",
		Password:  "secret",
	})
	if err != nil {
		log.Fatal(err)
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
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.Total)
}
