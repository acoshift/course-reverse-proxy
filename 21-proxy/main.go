package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	{
		caKeyBytes, _ := ioutil.ReadFile("ca.key")
		caKeyPem, _ := pem.Decode(caKeyBytes)
		caPriv, err := x509.ParseECPrivateKey(caKeyPem.Bytes)
		if err != nil {
			log.Fatal(err)
		}
		caCrtBytes, _ := ioutil.ReadFile("ca.crt")
		caCrtPem, _ := pem.Decode(caCrtBytes)
		caCrt, err := x509.ParseCertificate(caCrtPem.Bytes)
		if err != nil {
			log.Fatal(err)
		}

		var cacheCrt = make(map[string]*tls.Certificate)

		srv := &http.Server{
			Handler: http.HandlerFunc(proxyHTTPS),
			TLSConfig: &tls.Config{
				MinVersion: tls.VersionTLS10,
				CurvePreferences: []tls.CurveID{
					tls.X25519,
					tls.CurveP256,
				},
				PreferServerCipherSuites: true,
				GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
					if _, ok := cacheCrt[info.ServerName]; ok {
						return cacheCrt[info.ServerName], nil
					}
					log.Println("generate cert", info.ServerName)

					privKey, _ := rsa.GenerateKey(rand.Reader, 2048)

					serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
					x509Crt := &x509.Certificate{
						Subject: pkix.Name{
							CommonName: info.ServerName,
						},
						Issuer:       caCrt.Issuer,
						SerialNumber: serial,
						NotBefore:    time.Now().UTC(),
						NotAfter:     time.Now().UTC().AddDate(1, 0, 0).UTC(),
						KeyUsage:     x509.KeyUsageDigitalSignature,
						DNSNames:     []string{info.ServerName},
						ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
					}
					crt, err := x509.CreateCertificate(rand.Reader, x509Crt, caCrt, &privKey.PublicKey, caPriv)
					if err != nil {
						return nil, err
					}

					cacheCrt[info.ServerName] = &tls.Certificate{
						Certificate: [][]byte{crt},
						PrivateKey:  privKey,
						Leaf:        x509Crt,
					}

					return cacheCrt[info.ServerName], nil
				},
			},
		}
		go srv.ServeTLS(&forwardConnListener{}, "", "")
	}

	http.ListenAndServe(":8888", http.HandlerFunc(proxy))
}

var tr = &http.Transport{}

func proxy(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Host, r.RequestURI)

	if r.Method == http.MethodConnect {
		tunnelConn(w, r)
		return
	}

	proxyHTTP(w, r)
}

var connCh = make(chan net.Conn)

type forwardConnListener struct{}

func (l *forwardConnListener) Accept() (net.Conn, error) {
	return <-connCh, nil
}

func (l *forwardConnListener) Close() error   { return nil }
func (l *forwardConnListener) Addr() net.Addr { return nil }

func tunnelConn(w http.ResponseWriter, r *http.Request) {
	// dstConn, err := net.Dial("tcp", "127.0.0.1:8889")
	// if err != nil {
	// 	http.Error(w, "Bad Gateway", http.StatusBadGateway)
	// 	return
	// }
	// defer dstConn.Close()

	srcConn, wr, _ := w.(http.Hijacker).Hijack()
	// defer srcConn.Close()

	wr.WriteString("HTTP/1.1 200 OK\r\n\r\n")
	wr.Flush()

	connCh <- srcConn
	// go io.Copy(dstConn, srcConn)
	// io.Copy(srcConn, dstConn)
}

func proxyHTTP(w http.ResponseWriter, r *http.Request) {
	r.Header.Del("Accept-Encoding")

	// if r.Body != nil {
	// 	b := r.Body
	// 	defer b.Close()
	// 	r.Body = ioutil.NopCloser(io.TeeReader(r.Body, os.Stdout))
	// }

	resp, err := tr.RoundTrip(r)
	if err != nil {
		log.Println(err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		io.Copy(w, io.TeeReader(resp.Body, os.Stdout))
	} else {
		io.Copy(w, resp.Body)
	}
}

func proxyHTTPS(w http.ResponseWriter, r *http.Request) {
	r.URL.Scheme = "https"
	r.URL.Host = r.Host
	proxyHTTP(w, r)
}
