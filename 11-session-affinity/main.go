package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
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
	h := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Upstream %d", port)
	}

	http.ListenAndServe(fmt.Sprintf(":%d", port), http.HandlerFunc(h))
}

var tr = &http.Transport{
	MaxIdleConnsPerHost: 10,
}

var rrlbIndex uint32

func handler(w http.ResponseWriter, r *http.Request) {
	// config upstream server
	r.URL.Scheme = "http"

	index := -1

	// get upstream index from cookie
	{
		c, _ := r.Cookie("session_affinity")
		if c != nil {
			var err error
			index, err = strconv.Atoi(c.Value)
			if err != nil {
				index = -1
			}
		}
	}

	// request don't have affinity
	if index == -1 {
		// get current upstream
		index = int(atomic.AddUint32(&rrlbIndex, 1))
	}

	// rolling cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_affinity",
		Value:    strconv.Itoa(index),
		Path:     "/",
		MaxAge:   int(7 * 24 * time.Hour / time.Second), // 7 days
		HttpOnly: true,
	})

	r.URL.Host = upstreams[index%len(upstreams)]

	// forward request to upstream
	resp, err := tr.RoundTrip(r)
	if err != nil {
		log.Println(err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}

	// copy response to client
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
