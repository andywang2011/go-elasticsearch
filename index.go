package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

// IndexAPI defines typed index operations.
type IndexAPI[T any] interface {
	IndexDoc(ctx context.Context, id string, body any) error
	Delete(ctx context.Context, id string) error
	Search(ctx context.Context, req SearchRequest) (*PaginatedResult[T], error)
	HybridSearch(ctx context.Context, req HybridSearchRequest) (*PaginatedResult[T], error)
}

// Index is a generic typed index handle.
type Index[T any] struct {
	name   string
	client *Client
}

// NewIndex creates a typed index handle for the given client and index name.
func NewIndex[T any](client *Client, indexName string) IndexAPI[T] {
	return &Index[T]{name: indexName, client: client}
}

// IndexDoc indexes or updates a document by ID.
func (i *Index[T]) IndexDoc(ctx context.Context, id string, body any) error {
	if i.client == nil {
		return fmt.Errorf("elasticsearch: client is not initialized")
	}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("elasticsearch: marshal document: %w", err)
	}
	req := esapi.IndexRequest{
		Index:      i.name,
		DocumentID: id,
		Body:       bytes.NewReader(data),
		Refresh:    i.client.refresh(),
	}
	resp, err := req.Do(ctx, i.client.client)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return fmt.Errorf("elasticsearch: index error: %s", resp.String())
	}
	return nil
}

// Delete removes a document by ID.
func (i *Index[T]) Delete(ctx context.Context, id string) error {
	if i.client == nil {
		return fmt.Errorf("elasticsearch: client is not initialized")
	}
	req := esapi.DeleteRequest{
		Index:      i.name,
		DocumentID: id,
		Refresh:    i.client.refresh(),
	}
	resp, err := req.Do(ctx, i.client.client)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.IsError() && resp.StatusCode != 404 {
		return fmt.Errorf("elasticsearch: delete error: %s", resp.String())
	}
	return nil
}

// Search runs a keyword search.
func (i *Index[T]) Search(ctx context.Context, req SearchRequest) (*PaginatedResult[T], error) {
	body, err := buildSearchBody(req.Size, map[string]any{
		"query": req.Query,
		"sort":  req.Sort,
	}, req.SearchAfter, req.Source)
	if err != nil {
		return nil, err
	}
	return i.searchBody(ctx, body)
}

// HybridSearch runs keyword + kNN with optional RRF fusion.
func (i *Index[T]) HybridSearch(ctx context.Context, req HybridSearchRequest) (*PaginatedResult[T], error) {
	payload := map[string]any{}
	if req.Query != nil {
		payload["query"] = req.Query
	}
	if len(req.KNN) > 0 {
		payload["knn"] = req.KNN
	}
	if req.Rank != nil {
		payload["rank"] = map[string]any{"rrf": req.Rank}
	}
	body, err := buildSearchBody(req.Size, payload, req.SearchAfter, req.Source)
	if err != nil {
		return nil, err
	}
	return i.searchBody(ctx, body)
}

func buildSearchBody(size int, fields map[string]any, searchAfter []any, source []string) ([]byte, error) {
	if size <= 0 {
		size = 20
	}
	body := map[string]any{"size": size}
	for k, v := range fields {
		if v == nil {
			continue
		}
		switch val := v.(type) {
		case []KNNQuery:
			if len(val) == 0 {
				continue
			}
			body[k] = val
		default:
			body[k] = v
		}
	}
	if len(searchAfter) > 0 {
		body["search_after"] = searchAfter
	}
	if len(source) > 0 {
		body["_source"] = source
	}
	return json.Marshal(body)
}

func (i *Index[T]) searchBody(ctx context.Context, body []byte) (*PaginatedResult[T], error) {
	if i.client == nil {
		return nil, fmt.Errorf("elasticsearch: client is not initialized")
	}
	esReq := esapi.SearchRequest{
		Index: []string{i.name},
		Body:  bytes.NewReader(body),
	}
	resp, err := esReq.Do(ctx, i.client.client)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.IsError() {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("elasticsearch: search error: %s", string(b))
	}
	var result Result[T]
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("elasticsearch: decode search response: %w", err)
	}
	return paginate(result)
}

func paginate[T any](result Result[T]) (*PaginatedResult[T], error) {
	sourceList := make([]T, len(result.Hits.Hits))
	var nextSort []types.FieldValue
	for k, hit := range result.Hits.Hits {
		sourceList[k] = hit.Source
		if k == len(result.Hits.Hits)-1 {
			nextSort = hit.Sort
		}
	}
	total := 0
	if result.Hits.Total.Value > 0 {
		total = result.Hits.Total.Value
	}
	return &PaginatedResult[T]{
		Total: total,
		List:  sourceList,
		Next:  nextSort,
	}, nil
}
