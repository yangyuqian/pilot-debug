package main

import (
	"flag"

	"fmt"
	"html"
	"log"
	"net/http"

	"golang.org/x/net/http2"
)

// command-line flags
var (
	serviceName = flag.String("name", "", "unique service name across overall kubernetes cluster")
	// serve both HTTP/1.1 and HTTP/2
	http1addr = flag.String("http1-addr", ":6000", "HTTP/1.1 adress")
	http2addr = flag.String("http2-addr", ":6001", "HTTP/2 address")

	// connect to HTTP/1.1 and HTTP/2
	http1target = flag.String("http1-target", "", "HTTP/1.1 target")
	http2target = flag.String("http2-target", "", "HTTP/2 target")

	// redis sentinels
	redisSentinels = flag.String("redis-sentinels", "", "redis sentinel address")

	// redis addr
	redisAddr = flag.String("redis-addr", "", "redis address")
)

func main() {
	if !flag.Parsed() {
		flag.Parse()
	}

	var server http.Server

	http2.VerboseLogs = false //set true for verbose console output
	server.Addr = *http2addr

	log.Printf("Starting server on %s...", *http2addr)

	http2.ConfigureServer(&server, nil)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "URL: %q\n", html.EscapeString(r.URL.Path))
		ShowRequestInfoHandler(w, r)
	})

	log.Fatal(server.ListenAndServe())
}

func ShowRequestInfoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	fmt.Fprintf(w, "Method: %s\n", r.Method)
	fmt.Fprintf(w, "Protocol: %s\n", r.Proto)
	fmt.Fprintf(w, "Host: %s\n", r.Host)
	fmt.Fprintf(w, "RemoteAddr: %s\n", r.RemoteAddr)
	fmt.Fprintf(w, "RequestURI: %q\n", r.RequestURI)
	fmt.Fprintf(w, "URL: %#v\n", r.URL)
	fmt.Fprintf(w, "Body.ContentLength: %d (-1 means unknown)\n", r.ContentLength)
	fmt.Fprintf(w, "Close: %v (relevant for HTTP/1 only)\n", r.Close)
	fmt.Fprintf(w, "TLS: %#v\n", r.TLS)
	fmt.Fprintf(w, "\nHeaders:\n")

	r.Header.Write(w)
}
