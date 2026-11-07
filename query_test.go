package elasticsearch

import (
	"encoding/json"
	"testing"
)

func TestMultiMatch(t *testing.T) {
	q := MultiMatch("keyword", []string{"title", "content"}, "and")
	b, _ := json.Marshal(q)
	m := make(map[string]any)
	_ = json.Unmarshal(b, &m)
	if _, ok := m["multi_match"]; !ok {
		t.Error("expected multi_match key")
	}
}

func TestTerm(t *testing.T) {
	q := Term("status", 1)
	if _, ok := q["term"]; !ok {
		t.Error("expected term key")
	}
}

func TestBoolQuery(t *testing.T) {
	must := []map[string]any{MultiMatch("x", []string{"a"}, "")}
	filter := []map[string]any{Term("status", 1)}
	q := BoolQuery(must, filter, nil)
	if _, ok := q["bool"]; !ok {
		t.Error("expected bool key")
	}
}

func TestDefaultRRF(t *testing.T) {
	rrf := DefaultRRF()
	if rrf.WindowSize != 100 || rrf.RankConstant != 60 {
		t.Errorf("unexpected rrf: %+v", rrf)
	}
}
