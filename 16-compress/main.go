package main

import (
	"compress/gzip"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/google/brotli/go/cbrotli"
)

func main() {
	h := chain(
		Compress(Gzip()),
		Compress(Br()),
	)(http.HandlerFunc(handler))
	http.ListenAndServe(":8080", h)
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum."))
}

func chain(hs ...func(h http.Handler) http.Handler) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		for i := len(hs); i > 0; i-- {
			h = hs[i-1](h)
		}
		return h
	}
}

type CompressConfig struct {
	New       func() Compressor
	Encoding  string // http Accept-Encoding, Content-Encoding value
	Vary      bool   // add Vary: Accept-Encoding
	Types     string // only compress for given types, * for all types
	MinLength int    // skip if Content-Length less than given value
}

func Gzip() CompressConfig {
	return CompressConfig{
		New: func() Compressor {
			g, err := gzip.NewWriterLevel(ioutil.Discard, gzip.DefaultCompression)
			if err != nil {
				panic(err)
			}
			return g
		},
		Encoding:  "gzip",
		Vary:      defaultCompressVary,
		Types:     defaultCompressTypes,
		MinLength: defaultCompressMinLength,
	}
}

func Br() CompressConfig {
	return CompressConfig{
		New: func() Compressor {
			return &brWriter{quality: 4}
		},
		Encoding:  "br",
		Vary:      defaultCompressVary,
		Types:     defaultCompressTypes,
		MinLength: defaultCompressMinLength,
	}
}

type brWriter struct {
	quality int
	*cbrotli.Writer
}

func (w *brWriter) Reset(p io.Writer) {
	w.Writer = cbrotli.NewWriter(p, cbrotli.WriterOptions{Quality: w.quality})
}

const (
	defaultCompressVary      = true
	defaultCompressTypes     = "application/xml+rss application/atom+xml application/javascript application/x-javascript application/json application/rss+xml application/vnd.ms-fontobject application/x-font-ttf application/x-web-app-manifest+json application/xhtml+xml application/xml font/opentype image/svg+xml image/x-icon text/css text/html text/plain text/x-component"
	defaultCompressMinLength = 860
)

func Compress(config CompressConfig) func(http.Handler) http.Handler {
	mapTypes := make(map[string]struct{})
	for _, t := range strings.Split(config.Types, " ") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		mapTypes[t] = struct{}{}
	}

	pool := &sync.Pool{
		New: func() interface{} {
			return config.New()
		},
	}

	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// skip if client not support
			if !strings.Contains(r.Header.Get("Accept-Encoding"), config.Encoding) {
				h.ServeHTTP(w, r)
				return
			}

			// skip if web socket
			if r.Header.Get("Sec-WebSocket-Key") != "" {
				h.ServeHTTP(w, r)
				return
			}

			hh := w.Header()

			// skip if already encode
			if hh.Get("Content-Encoding") != "" {
				h.ServeHTTP(w, r)
				return
			}

			if config.Vary {
				hh.Add("Vary", "Accept-Encoding")
			}

			cw := &compressWriter{
				ResponseWriter: w,
				pool:           pool,
				encoding:       config.Encoding,
				types:          mapTypes,
				minLength:      config.MinLength,
			}
			defer cw.Close()

			h.ServeHTTP(cw, r)
		})
	}
}

type Compressor interface {
	io.Writer
	io.Closer
	Reset(io.Writer)
	Flush() error
}

type compressWriter struct {
	http.ResponseWriter
	pool        *sync.Pool
	encoder     Compressor
	encoding    string
	types       map[string]struct{}
	wroteHeader bool
	minLength   int
}

func (w *compressWriter) init() {
	h := w.Header()

	// skip if already encode
	if h.Get("Content-Encoding") != "" {
		return
	}

	// skip if length < min length
	if w.minLength > 0 {
		if sl := h.Get("Content-Length"); sl != "" {
			l, _ := strconv.Atoi(sl)
			if l > 0 && l < w.minLength {
				return
			}
		}
	}

	// skip if no match type
	if _, ok := w.types["*"]; !ok {
		ct, _, err := mime.ParseMediaType(h.Get("Content-Type"))
		if err != nil {
			ct = "application/octet-stream"
		}
		if _, ok := w.types[ct]; !ok {
			return
		}
	}

	w.encoder = w.pool.Get().(Compressor)
	w.encoder.Reset(w.ResponseWriter)
	h.Del("Content-Length")
	h.Set("Content-Encoding", w.encoding)
}

func (w *compressWriter) Close() {
	if w.encoder == nil {
		return
	}
	w.encoder.Close()
	w.pool.Put(w.encoder)
}

func (w *compressWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.encoder != nil {
		return w.encoder.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

func (w *compressWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.init()
	w.ResponseWriter.WriteHeader(code)
}
