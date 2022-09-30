package mock

import "net/http"

// ResettableHandler is an HTTP handler that can reset its internal state.
type ResettableHandler interface {
	http.Handler
	Reset()
}
