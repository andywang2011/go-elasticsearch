package elasticsearch

// Query helpers for building SearchRequest and HybridSearchRequest bodies.

// MultiMatch builds a multi_match query; operator may be "and" or "or".
func MultiMatch(query string, fields []string, operator string) map[string]any {
	m := map[string]any{"query": query, "fields": fields}
	if operator != "" {
		m["operator"] = operator
	}
	return map[string]any{"multi_match": m}
}

// Term builds an exact term query.
func Term(field string, value any) map[string]any {
	return map[string]any{
		"term": map[string]any{field: value},
	}
}

// Terms builds a terms query for multiple exact values.
func Terms(field string, values any) map[string]any {
	return map[string]any{
		"terms": map[string]any{field: values},
	}
}

// RangeQuery builds a range query with optional gte and lt bounds.
func RangeQuery(field string, gte, lt any) map[string]any {
	r := map[string]any{}
	if gte != nil {
		r["gte"] = gte
	}
	if lt != nil {
		r["lt"] = lt
	}
	return map[string]any{
		"range": map[string]any{field: r},
	}
}

// BoolQuery combines must, filter, and should clauses.
func BoolQuery(must, filter, should []map[string]any) map[string]any {
	b := map[string]any{}
	if len(must) > 0 {
		b["must"] = must
	}
	if len(filter) > 0 {
		b["filter"] = filter
	}
	if len(should) > 0 {
		b["should"] = should
	}
	return map[string]any{"bool": b}
}

// DefaultRRF returns a sensible RRF config for hybrid search.
func DefaultRRF() *RRFConfig {
	return &RRFConfig{WindowSize: 100, RankConstant: 60}
}
