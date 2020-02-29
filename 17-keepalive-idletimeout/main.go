package main

import (
	"log"
	"net"
	"net/http"
	"time"
)

func main() {
	srv := &http.Server{
		Addr:        ":8080",
		IdleTimeout: 5 * time.Second,
		ConnState: func(conn net.Conn, state http.ConnState) {
			log.Println(conn.RemoteAddr(), state)
		},
	}

	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello"))
	})
	srv.ListenAndServe()
}
