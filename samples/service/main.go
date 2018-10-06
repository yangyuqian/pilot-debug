package main

import (
	"flag"

	"fmt"
	"html"
	"log"
	"net"
	"net/http"
	"time"

	"io/ioutil"
	"strings"

	"errors"
	"github.com/go-redis/redis"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	mockproto "github.com/yangyuqian/pilot-debug/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"math/rand"
)

// command-line flags
var (
	serviceName = flag.String("name", "", "unique service name across overall kubernetes cluster")
	// serve both HTTP/1.1 and HTTP/2
	http1addr = flag.String("http1-addr", ":6000", "HTTP/1.1 adress")
	http2addr = flag.String("http2-addr", ":6001", "HTTP/2 adress")

	// connect to HTTP/1.1 and HTTP/2
	http1targets = flag.String("http1-target", "", "HTTP/1.1 target")
	http2target  = flag.String("http2-target", "", "HTTP/2 target")

	// redis addr
	redisAddr = flag.String("redis-addr", "", "redis address")
	// random error
	errorRatio = flag.Int("error-ratio", 50, "ratio to generate errors")
	// random latency in milliseconds
	latency    = flag.Int("latency-milliseconds", 500, "set upper bound of the random latency")
	headerlist = []string{"x-request-id", "x-b3-traceid", "x-b3-spanid", "x-b3-parentspanid", "x-b3-sampled", "x-b3-flags", "x-ot-span-context", "x-my-user"}
)

// clients
var (
	http1client *http.Client
	redisclient *redis.Client
	http2client mockproto.MockClient
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

type mockGrpcServer struct{}

func (s *mockGrpcServer) Say(ctx context.Context, req *mockproto.MockRequest) (*mockproto.MockReply, error) {
	// response 5xx randomly between [0, 2 x errorRatio)
	if *errorRatio > 0 {
		log.Printf("accessing gRPC with error Ratio %d ...", *errorRatio)
		ratio := rand.Intn(2 * *errorRatio)
		if *errorRatio >= 100 || ratio >= *errorRatio {
			return nil, errors.New("generated error...")
		}
	}

	time.Sleep(time.Duration(rand.Intn(*latency*2)) * time.Millisecond)
	return &mockproto.MockReply{Msg: fmt.Sprintf("got message %s", req.Msg)}, nil
}

func registerMockGrpcServer(mockSrv mockproto.MockServer) (grpcServer *grpc.Server) {
	grpcServer = grpc.NewServer()
	mockproto.RegisterMockServer(grpcServer, mockSrv)
	return
}

func main() {
	if !flag.Parsed() {
		flag.Parse()
	}

	if *http1targets != "" {
		http1client = http.DefaultClient
	}

	if *http2target != "" {
		var opts []grpc.DialOption

		opts = append(opts, grpc.WithInsecure())

		conn, err := grpc.Dial(*http2target, opts...)
		if err != nil {
			log.Printf("cannot connect to %s", *http2target)
		}

		http2client = mockproto.NewMockClient(conn)
	}

	if *redisAddr != "" {
		redisclient = redis.NewClient(&redis.Options{
			Addr: *redisAddr,
		})
	}

	http.HandleFunc("/metrics", promhttp.Handler().ServeHTTP)

	http.HandleFunc("/", promhttp.InstrumentHandlerCounter(httpReqCnt, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// response 5xx randomly between [0, 2 x errorRatio)
		if *errorRatio > 0 {
			log.Printf("accessing HTTP/1.1 with error Ratio %d ...", *errorRatio)
			ratio := rand.Intn(2 * *errorRatio)
			if *errorRatio >= 100 || ratio >= *errorRatio {
				panic("generated 502 error...")
			}
		}
		time.Sleep(time.Duration(rand.Intn(*latency*2)) * time.Millisecond)

		if http1client != nil {
			fmt.Fprintln(w, "------------------------ BEGIN HTTP/1.1 ------------------------")
			f := func() {
				for _, http1target := range strings.Split(*http1targets, ",") {
					// GET.http1target
					parts := strings.Split(http1target, "|")
					req, err := http.NewRequest(parts[0], parts[1], nil)
					if err != nil {
						panic(err)
					}
					req.Header = make(http.Header)
					for _, it := range headerlist {
						if ihdr := r.Header.Get(it); ihdr != "" {
							req.Header.Set(it, ihdr)
						}
					}

					resp, err := http1client.Do(req)
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
			}

			f()
			fmt.Fprintln(w, "------------------------ END HTTP/1.1 ------------------------")
		}

		if http2client != nil {
			headers := make(map[string]string)
			for _, it := range headerlist {
				if ihdr := r.Header.Get(it); ihdr != "" {
					headers[it] = ihdr
				}
			}
			md := metadata.New(headers)

			resp, err := http2client.Say(context.TODO(), &mockproto.MockRequest{Msg: fmt.Sprintf("this is %s", *serviceName)}, grpc.Header(&md))
			if err != nil {
				log.Printf("cannot say hello to grpc service %+v", err)
			} else {
				fmt.Fprintf(w, "------------------------ gRPC response %s ------------------------", resp.Msg)
			}
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
		log.Printf("serving HTTP/1.1 on %s...", *http1addr)
		log.Fatal(http.ListenAndServe(*http1addr, nil))
	}()

	lv2, err := net.Listen("tcp", *http2addr)
	if err != nil {
		log.Fatal("cannot listen on %s", *http2addr)
	}

	grpcServer := registerMockGrpcServer(&mockGrpcServer{})
	grpcServer.Serve(lv2)

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
}
