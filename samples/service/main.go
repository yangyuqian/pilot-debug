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

	"errors"
	"github.com/go-redis/redis"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	mockproto "github.com/yangyuqian/pilot-debug/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"math/rand"
)

// command-line flags
var (
	serviceName = flag.String("name", "", "unique service name across overall kubernetes cluster")
	// serve both HTTP/1.1 and HTTP/2
	http1addr = flag.String("http1-addr", ":6000", "HTTP/1.1 adress")
	http2addr = flag.String("http2-addr", ":6001", "HTTP/2 adress")

	// connect to HTTP/1.1 and HTTP/2
	http1target = flag.String("http1-target", "", "HTTP/1.1 target")
	http2target = flag.String("http2-target", "", "HTTP/2 target")

	// redis addr
	redisAddr = flag.String("redis-addr", "", "redis address")
	// random error
	errorRatio = flag.Int("error-ratio", 50, "ratio to generate errors")
	// random latency in milliseconds
	latency = flag.Int("latency-milliseconds", 500, "set upper bound of the random latency")
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
		if ratio >= *errorRatio {
			return nil, errors.New("generated error...")
		}
	}

	time.Sleep(time.Duration(*latency) * time.Millisecond)
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

	if *http1target != "" {
		http1client = http.DefaultClient
	}

	if *http2target != "" {
		conn, err := grpc.Dial(*http2target, grpc.WithInsecure())
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
			if ratio >= *errorRatio {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
		time.Sleep(time.Duration(*latency) * time.Millisecond)

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
			resp, err := http2client.Say(context.TODO(), &mockproto.MockRequest{Msg: fmt.Sprintf("this is %s", *serviceName)})
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
