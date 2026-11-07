// Hybrid search example (bool query + kNN + RRF). Requires a running Elasticsearch cluster.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	libes "github.com/andywang2011/go-elasticsearch"
)

func main() {
	addr := env("ES_ADDRESS", "http://localhost:9200")
	client, err := libes.NewClient(libes.Config{
		Addresses: []string{addr},
		Username:  os.Getenv("ES_USERNAME"),
		Password:  os.Getenv("ES_PASSWORD"),
		Timeout:   15 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}

	type Doc struct {
		Title string    `json:"title"`
		Emb   []float32 `json:"embedding"`
	}

	idx := libes.NewIndex[Doc](client, env("ES_INDEX", "articles"))
	vector := make([]float32, 8)
	ctx := context.Background()
	result, err := idx.HybridSearch(ctx, libes.HybridSearchRequest{
		Size: 10,
		Query: libes.BoolQuery(
			[]map[string]any{libes.MultiMatch("features", []string{"title"}, "and")},
			nil,
			nil,
		),
		KNN: []libes.KNNQuery{{
			Field:         "embedding",
			QueryVector:   vector,
			K:             5,
			NumCandidates: 50,
		}},
		Rank: libes.DefaultRRF(),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("hybrid hits total=%d\n", result.Total)
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
