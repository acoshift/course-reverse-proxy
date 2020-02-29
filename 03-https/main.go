package main

import (
	"io"
	"log"
	"net/http"
)

// openssl req -x509 -newkey rsa:4096 -nodes -keyout server.key -out server.crt -days 365 -subj '/CN=example.com'
func main() {
	http.ListenAndServeTLS(
		":8080",
		"server.crt", "server.key",
		http.HandlerFunc(handler),
	)
}

func handler(w http.ResponseWriter, r *http.Request) {
	// config upstream server
	r.URL.Scheme = "http"
	r.URL.Host = "93.184.216.34"

	// forward request to upstream
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		log.Println(err)
		return
	}

	// copy response to client
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
