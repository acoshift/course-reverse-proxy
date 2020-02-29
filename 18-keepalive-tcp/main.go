package main

import (
	"net"
	"net/http"
	"time"
)

func main() {
	srv := &http.Server{}

	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello"))
	})

	tcpListener, _ := net.Listen("tcp", ":8080")
	keepAliveListener := &keepAliveTCPListener{
		TCPListener: tcpListener.(*net.TCPListener),
	}
	srv.Serve(keepAliveListener)
}

type keepAliveTCPListener struct {
	*net.TCPListener
}

func (l *keepAliveTCPListener) Accept() (net.Conn, error) {
	conn, err := l.TCPListener.Accept()
	if err != nil {
		return nil, err
	}
	conn.(*net.TCPConn).SetKeepAlive(true)
	conn.(*net.TCPConn).SetKeepAlivePeriod(5 * time.Second)
	return conn, nil
}
