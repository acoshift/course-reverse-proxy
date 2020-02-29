package main

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {
	h := logMiddleware(http.HandlerFunc(handler))
	http.ListenAndServe(":8080", h)
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello"))
}

func logMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timestamp := time.Now()
		nw := &logResponseWriter{
			ResponseWriter: w,
		}
		defer func() {
			diff := time.Since(timestamp)
			remoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)
			json.NewEncoder(os.Stdout).Encode(struct {
				Timestamp string `json:"timestamp"`
				Status    int    `json:"status"`
				Method    string `json:"method"`
				URI       string `json:"uri"`
				Host      string `json:"host"`
				RemoteIP  string `json:"remote_ip"`
				Duration  string `json:"duration"`
				BytesIn   int64  `json:"bytes_in"`
				BytesOut  int64  `json:"bytes_out"`
			}{
				timestamp.Format(time.RFC3339),
				nw.statusCode,
				r.Method,
				r.RequestURI,
				r.Host,
				remoteIP,
				diff.String(),
				r.ContentLength,
				nw.sentBytes,
			})
		}()

		h.ServeHTTP(nw, r)
	})
}

type logResponseWriter struct {
	http.ResponseWriter

	wroteHeader bool
	statusCode  int
	sentBytes   int64
}

func (w *logResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.statusCode = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *logResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	w.sentBytes += int64(n)
	return n, err
}
