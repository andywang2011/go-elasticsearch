package elasticsearch

import (
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

// TotalHits is the hits.total object: {"value": N, "relation": "eq"|"gte"}.
// Legacy servers that return a bare number are accepted as relation "eq".
type TotalHits struct {
	// Value is the hit count.
	Value int `json:"value"`
	// Relation is "eq" for an exact count or "gte" when the count is a lower bound.
	Relation string `json:"relation"`
}

// UnmarshalJSON accepts both the {value, relation} object and a bare number.
func (t *TotalHits) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	if len(data) > 0 && data[0] != '{' {
		var n int
		if err := json.Unmarshal(data, &n); err != nil {
			return fmt.Errorf("elasticsearch: parse hits.total: %w", err)
		}
		t.Value, t.Relation = n, "eq"
		return nil
	}
	type plain TotalHits
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("elasticsearch: parse hits.total: %w", err)
	}
	*t = TotalHits(p)
	return nil
}

// Result is the generic Elasticsearch search response envelope.
type Result[T any] struct {
	Hits struct {
		Total TotalHits `json:"total"`
		Hits  []struct {
			ID     string             `json:"_id"`
			Source T                  `json:"_source"`
			Sort   []types.FieldValue `json:"sort,omitempty"`
		} `json:"hits"`
	} `json:"hits"`
}

// PaginatedResult is a unified paginated search result.
type PaginatedResult[T any] struct {
	// Total is the hit count reported by Elasticsearch.
	Total int `json:"total"`
	// TotalRelation is "eq" when Total is exact or "gte" when it is a lower bound.
	TotalRelation string `json:"total_relation,omitempty"`
	// List contains deserialized document sources.
	List []T `json:"list"`
	// Next holds search_after sort keys for the next page.
	Next []types.FieldValue `json:"next,omitempty"`
}

// SearchRequest is a keyword search request body.
type SearchRequest struct {
	// Size is the page size; defaults to 20 when zero.
	Size int `json:"size,omitempty"`
	// Query is the Elasticsearch query DSL object.
	Query map[string]any `json:"query,omitempty"`
	// Sort is the sort definition array.
	Sort []map[string]any `json:"sort,omitempty"`
	// SearchAfter is the cursor for deep pagination.
	SearchAfter []any `json:"search_after,omitempty"`
	// Source limits returned _source fields.
	Source []string `json:"_source,omitempty"`
}

// KNNQuery is one kNN clause for dense_vector similarity search.
type KNNQuery struct {
	// Field is the dense_vector field name (e.g. "embedding").
	Field string `json:"field"`
	// QueryVector is the query embedding.
	QueryVector []float32 `json:"query_vector"`
	// K is the number of nearest neighbors to return.
	K int `json:"k"`
	// NumCandidates is the HNSW candidate pool size (must be >= K).
	NumCandidates int `json:"num_candidates,omitempty"`
	// Filter is an optional bool filter applied inside the kNN clause.
	Filter map[string]any `json:"filter,omitempty"`
}

// RRFConfig configures reciprocal rank fusion for hybrid search.
type RRFConfig struct {
	// WindowSize is the rank window for RRF (ES default 100).
	WindowSize int `json:"window_size,omitempty"`
	// RankConstant is the RRF rank constant (ES default 60).
	RankConstant int `json:"rank_constant,omitempty"`
}

// HybridSearchRequest combines keyword query, kNN, and optional RRF ranking.
type HybridSearchRequest struct {
	// Size is the page size; defaults to 20 when zero.
	Size int `json:"size,omitempty"`
	// Query is the keyword/bool query portion.
	Query map[string]any `json:"query,omitempty"`
	// KNN lists kNN clauses against dense_vector fields.
	KNN []KNNQuery `json:"knn,omitempty"`
	// Rank enables RRF fusion when set (typically with both Query and KNN).
	Rank *RRFConfig `json:"rank,omitempty"`
	// SearchAfter is the cursor for deep pagination.
	SearchAfter []any `json:"search_after,omitempty"`
	// Source limits returned _source fields.
	Source []string `json:"_source,omitempty"`
}
