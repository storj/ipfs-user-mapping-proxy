package mock

import "net/http"

// NoopHandler is an HTTP handler that does not do anything.
type NoopHandler struct{}

func (h *NoopHandler) Reset() {}

func (h *NoopHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {}
