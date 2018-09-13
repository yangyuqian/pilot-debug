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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

// clients
var (
	http1client, http2client    *http.Client
	redisclient, sentinelclient *redis.Client
)

// metrics
var (
	httpReqCnt = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "evangelist",
		Subsystem: "inbound_http",
		Name:      "request_stats",
		Help:      "stats requests",
	}, []string{"method", "code"})

	// dependency services, including HTTP/1.1, HTTP/2, Redis and Redis Sentinel
	http1DepCnt = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "evangelist",
		Subsystem: "outbound_http1",
		Name:      "result_stats",
		Help:      "stats HTTP/1.1",
	}, []string{"node", "state"})
	http2DepCnt = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "evangelist",
		Subsystem: "outbound_http2",
		Name:      "result_stats",
		Help:      "stats HTTP/2",
	}, []string{"node", "state"})
	redisDepCnt = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "evangelist",
		Subsystem: "outbound_redis",
		Name:      "result_stats",
		Help:      "stats Redis",
	}, []string{"node", "state"})
	sentinelDepCnt = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "evangelist",
		Subsystem: "outbound_sentinel",
		Name:      "result_stats",
		Help:      "stats Redis Sentinel",
	}, []string{"node", "state"})
)

func init() {
	prometheus.Register(httpReqCnt)
	prometheus.Register(http1DepCnt)
	prometheus.Register(http2DepCnt)
	prometheus.Register(redisDepCnt)
	prometheus.Register(sentinelDepCnt)
}

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

	http.HandleFunc("/metrics", promhttp.Handler().ServeHTTP)

	http.HandleFunc("/", promhttp.InstrumentHandlerCounter(httpReqCnt, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if http1client != nil {
			fmt.Fprintln(w, "------------------------ BEGIN HTTP/1.1 ------------------------")
			f := func() {
				resp, err := http1client.Get(*http1target)
				if err != nil {
					log.Printf("error to fetch info from http1 target %+v", err)
					http1DepCnt.WithLabelValues(*serviceName, "ko").Inc()
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
					http1DepCnt.WithLabelValues(*serviceName, "ko").Inc()
					return
				}

				http1DepCnt.WithLabelValues(*serviceName, "ok").Inc()
				w.Write(data)
			}

			f()
			fmt.Fprintln(w, "------------------------ END HTTP/1.1 ------------------------")
		}

		if http2client != nil {
			fmt.Fprintln(w, "------------------------ BEGIN HTTP/2 ------------------------")
			f := func() {
				resp, err := http2client.Get(*http2target)
				if err != nil {
					log.Printf("error to fetch info from http2 target %+v", err)
					http2DepCnt.WithLabelValues(*serviceName, "ko").Inc()
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
					http2DepCnt.WithLabelValues(*serviceName, "ko").Inc()
					return
				}

				http2DepCnt.WithLabelValues(*serviceName, "ok").Inc()
				w.Write(data)
			}

			f()
			fmt.Fprintln(w, "------------------------ END HTTP/2 ------------------------")
		}

		fmt.Fprintln(w, "------------------------ BEGIN SELF ------------------------")
		fmt.Fprintf(w, "URL: %q\n", html.EscapeString(r.URL.Path))
		ShowRequestInfoHandler(w, r)
		fmt.Fprintln(w, "------------------------ END SELF ------------------------")

		fmt.Fprintln(w, "------------------------ BEGIN REDIS ------------------------")
		CheckRedis(w)
		fmt.Fprintln(w, "------------------------ END REDIS ------------------------")
	})).ServeHTTP)

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

	fmt.Fprintf(w, "Node: %s\n", *serviceName)
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
			redisDepCnt.WithLabelValues(*serviceName, "ko").Inc()
			return
		}

		redisDepCnt.WithLabelValues(*serviceName, "ok").Inc()
		fmt.Fprintf(w, fmt.Sprintf("%s -> %s\n", k, redisclient.Get(k).Val()))
	}

	if sentinelclient != nil {
		k := fmt.Sprintf("sentinel:%s", *serviceName)
		if cmd := sentinelclient.Set(k, "hello", time.Duration(1)*time.Hour); cmd.Err() != nil {
			log.Printf("cannot set to sentinel %+v", cmd.Err())
			sentinelDepCnt.WithLabelValues(*serviceName, "ko").Inc()
			return
		}

		sentinelDepCnt.WithLabelValues(*serviceName, "ok").Inc()
		fmt.Fprintf(w, fmt.Sprintf("%s -> %s\n", k, sentinelclient.Get(k).Val()))
	}
}
