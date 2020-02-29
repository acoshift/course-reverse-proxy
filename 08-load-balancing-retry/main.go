package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"sync/atomic"
)

var upstreams []string

func main() {
	for i := 0; i < 3; i++ {
		port := 9000 + i
		upstreams = append(upstreams, fmt.Sprintf("127.0.0.1:%d", port))
		go startUpstream(port)
	}

	http.ListenAndServe(":8080", http.HandlerFunc(handler))
}

func startUpstream(port int) {
	if port%2 == 1 {
		return
	}

	h := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Upstream %d", port)
	}

	http.ListenAndServe(fmt.Sprintf(":%d", port), http.HandlerFunc(h))
}

var rrlbIndex uint32

func handler(w http.ResponseWriter, r *http.Request) {
	// config upstream server
	r.URL.Scheme = "http"

	// retry 3 times
	for i := 0; i < 3; i++ {
		// get current upstream
		index := int(atomic.AddUint32(&rrlbIndex, 1))
		r.URL.Host = upstreams[index%len(upstreams)]

		// forward request to upstream
		resp, err := http.DefaultTransport.RoundTrip(r)
		if err != nil {
			log.Println(err)
			continue
		}

		// copy response to client
		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		return
	}

	http.Error(w, "Bad Gateway", http.StatusBadGateway)
}
