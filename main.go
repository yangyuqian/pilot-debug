package main

import (
	"encoding/json"
	"flag"
	"fmt"
	apiv2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	discoveryv2 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

var (
	target = flag.String("target", "127.0.0.1:18000", "target address")
	addr   = flag.String("addr", ":9010", "address")
)

func main() {
	if !flag.Parsed() {
		flag.Parse()
	}

	conn, err := grpc.Dial(*target,
		grpc.WithInsecure(),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                time.Duration(5) * time.Second,
			Timeout:             time.Duration(3) * time.Second,
			PermitWithoutStream: true,
		}))
	if err != nil {
		panic(err)
	}

	cli := discoveryv2.NewAggregatedDiscoveryServiceClient(conn)

	stream, err := cli.StreamAggregatedResources(context.Background())
	if err != nil {
		panic(err)
	}

	go func() {
		if err := http.ListenAndServe(*addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, err := ioutil.ReadAll(r.Body)
			if err != nil {
				log.Printf("%+v", err)
				return
			}
			switch r.URL.Path {
			case "/ads":
				req := &apiv2.DiscoveryRequest{}
				if err := json.Unmarshal(raw, req); err != nil {
					log.Printf("%+v", err)
					return
				}

				if err := stream.Send(req); err != nil {
					log.Printf("[ERR] %+v", err)
				}
			}
		})); err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		for {
			resp, err := stream.Recv()
			if err != nil {
				log.Printf("[ERR] %+v", err)
			}
			fmt.Printf("--------------- VERSION: %s ------ TYPE_URL: %s --------------------------\n", resp.VersionInfo, resp.TypeUrl)
			switch resp.TypeUrl {
			case "type.googleapis.com/envoy.api.v2.Cluster":
				for _, v := range resp.Resources {
					vv := &apiv2.Cluster{}
					if err := proto.Unmarshal(v.Value, vv); err != nil {
						log.Printf("%+v", err)
					}
					fmt.Printf("|- %s: %s\n", v.TypeUrl, json2text(vv))
				}
			case "type.googleapis.com/envoy.api.v2.Listener":
				for _, v := range resp.Resources {
					vv := &apiv2.Listener{}
					if err := proto.Unmarshal(v.Value, vv); err != nil {
						log.Printf("%+v", err)
					}
					fmt.Printf("|- %s: %s\n", v.TypeUrl, json2text(vv))
				}

			case "type.googleapis.com/envoy.api.v2.ClusterLoadAssignment":
				if len(resp.Resources) == 0 {
					fmt.Printf("%s", json2text(resp))
				}

				for _, v := range resp.Resources {
					vv := &apiv2.ClusterLoadAssignment{}
					if err := proto.Unmarshal(v.Value, vv); err != nil {
						log.Printf("%+v", err)
					}
					fmt.Printf("|- %s: %s\n", v.TypeUrl, json2text(vv))
				}

			case "type.googleapis.com/envoy.api.v2.RouteConfiguration":
				if len(resp.Resources) == 0 {
					fmt.Printf("%s", json2text(resp))
				}

				for _, v := range resp.Resources {
					vv := &apiv2.RouteConfiguration{}
					if err := proto.Unmarshal(v.Value, vv); err != nil {
						log.Printf("%+v", err)
					}
					fmt.Printf("|- %s: %s\n", v.TypeUrl, json2text(vv))
				}
			}
		}
	}()

	select {}
}

func json2text(obj interface{}) string {
	o, err := json.MarshalIndent(obj, "  ", "  ")
	if err != nil {
		panic(err)
	}
	return string(o)
}
