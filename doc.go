// Package elasticsearch is a testable Elasticsearch client for document CRUD,
// keyword search, and hybrid retrieval (bool query + kNN + RRF).
//
// Design constraints:
//   - Configuration via Config (no viper, no global singletons, no log.Fatal).
//   - Generic Index[T] for typed documents and paginated search results.
//   - NewClient returns Client (client_iface.go); use ConfigDefault() for timeout/retry defaults.
//   - Injectable HTTP transport via NewClientWithTransport for unit tests.
//   - Embedding vectors are supplied by the application; this library does not generate them.
package elasticsearch
