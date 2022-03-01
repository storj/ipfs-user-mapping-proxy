package proxy

import "net/http"

// ResponseWriterWrapper wraps a ResponseWriter and makes a copy of its
// StatusCode and Body in public fields.
type ResponseWriterWrapper struct {
	http.ResponseWriter
	StatusCode int
	Body       []byte
}

// NewResponseWriterWrapper wraps the provided ResponseWrapper.
func NewResponseWriterWrapper(w http.ResponseWriter) *ResponseWriterWrapper {
	return &ResponseWriterWrapper{ResponseWriter: w, StatusCode: http.StatusOK}
}

func (rww *ResponseWriterWrapper) WriteHeader(statusCode int) {
	rww.StatusCode = statusCode
	rww.ResponseWriter.WriteHeader(statusCode)
}

func (rww *ResponseWriterWrapper) Write(b []byte) (int, error) {
	rww.Body = append(rww.Body, b...)
	return rww.ResponseWriter.Write(b)
}
