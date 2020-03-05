package main

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"net/http/httputil"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	go startH2CServer()

	rp := &httputil.ReverseProxy{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (conn net.Conn, err error) {
				log.Println("dial")
				return net.Dial(network, addr)
			},
		},
		Director: func(r *http.Request) {
			r.URL.Host = "127.0.0.1:9000"
			r.URL.Scheme = "http"
		},
	}
	http.ListenAndServe(":8080", rp)
}

func startH2CServer() {
	h := h2c.NewHandler(http.HandlerFunc(handler), &http2.Server{})
	http.ListenAndServe(":9000", h)
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello"))
}
