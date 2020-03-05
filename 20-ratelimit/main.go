package main

import (
	"net/http"
	"time"

	"github.com/moonrhythm/parapet"
	"github.com/moonrhythm/parapet/pkg/ratelimit"
)

func main() {
	srv := parapet.NewBackend()
	srv.Use(ratelimit.ConcurrentQueue(2, 100))
	srv.Handler = http.HandlerFunc(handler)
	srv.Addr = ":8080"
	srv.ListenAndServe()
}

func handler(w http.ResponseWriter, r *http.Request) {
	time.Sleep(time.Second)
	w.Write([]byte("Hello"))
}
