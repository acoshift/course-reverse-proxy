package main

import (
	"crypto/tls"
	"io"
	"log"
	"net/http"
)

// openssl req -x509 -newkey rsa:4096 -nodes -keyout server1.key -out server1.crt -days 365 -subj '/CN=example.com'
// openssl req -x509 -newkey rsa:4096 -nodes -keyout server2.key -out server2.crt -days 365 -subj '/CN=example.net'
func main() {
	tlsConfig := &tls.Config{}

	certFiles := []string{
		"server1",
		"server2",
	}
	for _, fn := range certFiles {
		cert, err := tls.LoadX509KeyPair(fn+".crt", fn+".key")
		if err != nil {
			log.Fatalf("can not load certificate; %v", err)
		}
		tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
	}

	tlsConfig.BuildNameToCertificate()

	srv := &http.Server{
		Addr:      ":8080",
		Handler:   http.HandlerFunc(handler),
		TLSConfig: tlsConfig,
	}

	srv.ListenAndServeTLS("", "")
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
