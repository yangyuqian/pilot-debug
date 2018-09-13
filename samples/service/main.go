package main

import (
	"flag"

	"fmt"
	"html"
	"log"
	"net/http"
	"strings"
	"time"

	"crypto/tls"
	"golang.org/x/net/http2"
	"io/ioutil"

	"github.com/go-redis/redis"
)

// command-line flags
var (
	serviceName = flag.String("name", "", "unique service name across overall kubernetes cluster")
	// serve both HTTP/1.1 and HTTP/2
	http2addr = flag.String("http2-addr", ":6000", "HTTP/2 adress")
	http1addr = flag.String("http1-addr", ":6001", "HTTP/1.1 adress")
	// tls key pairs
	certPath = flag.String("cert-path", "", "path of cert file")
	keyPath  = flag.String("key-path", "", "path of key file")

	// connect to HTTP/1.1 and HTTP/2
	http1target = flag.String("http1-target", "", "HTTP/1.1 target")
	http2target = flag.String("http2-target", "", "HTTP/2 target")

	// redis sentinels
	redisSentinelMaster = flag.String("redis-sentinel-master", "mymaster", "master name of sentinel based cluster")
	redisSentinelAddrs  = flag.String("redis-sentinel-addrs", "", "redis sentinel addresses separated by quotas")

	// redis addr
	redisAddr = flag.String("redis-addr", "", "redis address")
)

var (
	http1client, http2client    *http.Client
	redisclient, sentinelclient *redis.Client
)

func main() {
	if !flag.Parsed() {
		flag.Parse()
	}

	if *http1target != "" {
		http1client = http.DefaultClient
	}

	if *http2target != "" {
		http2client = &http.Client{
			Transport: &http2.Transport{
				AllowHTTP: true,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
	}

	if *redisSentinelAddrs != "" {
		sentinelclient = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    *redisSentinelMaster,
			SentinelAddrs: strings.Split(*redisSentinelAddrs, ","),
		})
	}

	if *redisAddr != "" {
		redisclient = redis.NewClient(&redis.Options{
			Addr: *redisAddr,
		})
	}

	http2.VerboseLogs = true //set true for verbose console output
	var server2 http.Server
	server2.Addr = *http2addr

	http2.ConfigureServer(&server2, nil)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "URL: %q\n", html.EscapeString(r.URL.Path))
		ShowRequestInfoHandler(w, r)

		if http1client != nil {
			resp, err := http1client.Get(*http1target)
			if err != nil {
				log.Printf("error to fetch info from http1 target %+v", err)
				return
			}
			defer func() {
				if resp.Body != nil {
					resp.Body.Close()
				}
			}()

			data, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Printf("error to fetch info from http1 target %+v", err)
			}

			w.Write(data)
		}

		if http2client != nil {
			resp, err := http2client.Get(*http2target)
			if err != nil {
				log.Printf("error to fetch info from http2 target %+v", err)
				return
			}

			defer func() {
				if resp.Body != nil {
					resp.Body.Close()
				}
			}()

			data, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Printf("error to fetch info from http2 target %+v", err)
			}

			w.Write(data)
		}

		CheckRedis(w)
	})

	go func() {
		log.Printf("serving HTTP/2 on %s...", *http2addr)
		log.Fatal(server2.ListenAndServeTLS(*certPath, *keyPath))
	}()

	go func() {
		log.Printf("serving HTTP/1.1 on %s...", *http1addr)
		log.Fatal(http.ListenAndServe(*http1addr, nil))
	}()

	select {}
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

func CheckRedis(w http.ResponseWriter) {
	if redisclient != nil {
		k := fmt.Sprintf("redis:%s", *serviceName)
		if cmd := redisclient.Set(k, "hello", time.Duration(1)*time.Hour); cmd.Err() != nil {
			log.Printf("cannot set to redis %+v", cmd.Err())
		}

		fmt.Fprintf(w, fmt.Sprintf("%s -> %s\n", k, redisclient.Get(k).Val()))
	}

	if sentinelclient != nil {
		k := fmt.Sprintf("sentinel:%s", *serviceName)
		if cmd := sentinelclient.Set(k, "hello", time.Duration(1)*time.Hour); cmd.Err() != nil {
			log.Printf("cannot set to sentinel %+v", cmd.Err())
		}
		fmt.Fprintf(w, fmt.Sprintf("%s -> %s\n", k, sentinelclient.Get(k).Val()))
	}
}
