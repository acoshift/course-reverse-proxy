package main

import (
	"fmt"
	"io"
	"log"
	"net"
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
	h := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Upstream %d\n", port)
		fmt.Fprintf(w, "XFF: %s\n", r.Header.Get("X-Forwarded-For"))
		fmt.Fprintf(w, "XFP: %s\n", r.Header.Get("X-Forwarded-Proto"))
		fmt.Fprintf(w, "Real IP: %s\n", r.Header.Get("X-Real-IP"))
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

	// get current upstream
	index := int(atomic.AddUint32(&rrlbIndex, 1))
	r.URL.Host = upstreams[index%len(upstreams)]

	// xf* headers
	r.Header.Set("X-Forwarded-Proto", protoFromRequest(r))
	r.Header.Set("X-Forwarded-For", remoteHostFromRequest(r))
	r.Header.Set("X-Real-IP", realIPFromRequest(r))

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

func protoFromRequest(r *http.Request) string {
	if r.TLS == nil {
		return "http"
	}
	return "https"
}

func remoteHostFromRequest(r *http.Request) string {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

var _, trustCIDR, _ = net.ParseCIDR("192.168.0.2/32")

func realIPFromRequest(r *http.Request) string {
	realIP := r.Header.Get("X-Real-IP")
	remoteIP := remoteHostFromRequest(r)

	if realIP == "" {
		return remoteIP
	}

	if trustCIDR.Contains(net.ParseIP(remoteIP)) {
		return realIP
	}

	return remoteIP
}
