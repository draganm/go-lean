package web

import "net/http"

type capturingResponseWriter struct {
	http.ResponseWriter
	status   int
	didWrite bool
}

func newCapturingResponseWriter(w http.ResponseWriter) *capturingResponseWriter {
	return &capturingResponseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
	}
}

func (crw *capturingResponseWriter) Write(b []byte) (int, error) {
	crw.didWrite = true
	return crw.ResponseWriter.Write(b)
}

func (crw *capturingResponseWriter) WriteHeader(statusCode int) {
	crw.ResponseWriter.WriteHeader(statusCode)

	if !crw.didWrite {
		crw.status = statusCode
		crw.didWrite = true
	}
}

func (crw *capturingResponseWriter) Unwrap() http.ResponseWriter {
	return crw.ResponseWriter
}
