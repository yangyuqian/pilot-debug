#!/bin/sh

function help() {
  cat << EOF
Check aDS settings via calling async gRPC endpoints of Pilot.

Usage:
  sh ads.sh [ARGS]

Example:
  sh ads.sh --pilot-target your-pilot-target

Available Arguments:
  --upgrade: upgrade pilot-debug server, calling gRPC endpoints directly to simulate xDS discovery with pilot
  --debug-server: start debug server connected with pilot server
  --pilot-target: pilot target address
  --check-clusters: check clusters
  --check-endpoints: check endpoints
  --check-listeners: check listeners
  --check-routes: check routes
  --node-id: node id
  --cluster: cluster name
  --selector: select pods by labels filters to collect metadata automatically
  --node-type: node type
  --resource-names: resource names, i.e. "a","b"
  --namespace: namespace
  --context: context
  --output: output format of istioctl, must be [short|json], default is short

EOF
  exit 0
}

function parse_args() {
  unknown=""
  while [[ $# -gt 0  ]]
  do
    keys="$1"
    case $keys in
      -h|--help)
        help
        shift
      ;;
      --upgrade)
        upgrade=true
        shift
      ;;
      --debug-server)
        debug_server=true
        shift
      ;;
      --pilot-target)
        pilot_target=$2
        shift
        shift
      ;;
      --check-clusters)
        check_clusters=true
        shift
      ;;
      --check-endpoints)
        check_endpoints=true
        shift
      ;;
      --check-listeners)
        check_listeners=true
        shift
      ;;
      --check-routes)
        check_routes=true
        shift
      ;;
      --node-id)
        node_id=$2
        shift
        shift
      ;;
      --cluster)
        cluster=$2
        shift
        shift
      ;;
      --node-type)
        node_type=$2
        shift
        shift
      ;;
      --selector)
        selector=$2
        shift
        shift
      ;;
      --context)
        context=$2
        shift
        shift
      ;;
      --namespace)
        namespace=$2
        shift
        shift
      ;;
      --resource-names)
        resource_names=$2
        shift
        shift
      ;;
      --output)
        output=$2
        shift
        shift
      ;;
      *)
        unknown="$unknown $1"
        shift
      ;;
    esac
  done

  if [ -n "$unknown" ]; then
    echo "unknown args $unknown"
    exit 1
  fi
}

parse_args $@

set -x
if [ "$upgrade" == "true" ]; then
  go get -u github.com/yangyuqian/pilot-debug
fi

if [ -z "$pilot_target" ]; then
  pilot_target=$PILOT_ADDR
fi

if [ -z "$output" ]; then
  output=short
fi

if [ "$debug_server" == "true" ]; then
  pilot-debug --target $pilot_target
fi

if [ -z "$namespace" ]; then
  namespace=default
fi

if [ -z "$context" ]; then
  context=ui.od.k8s.local
fi

if [ -n "$selector" ]; then
  pod_id=$(kubectl get pods --namespace $namespace --context $context --selector="$selector" -o jsonpath='{ .items[0].metadata.name }')

  if [ -z "$node_id" ]; then
    node_id=$(kubectl exec --namespace $namespace -it --context $context -c istio-proxy $pod_id -- cat /etc/istio/proxy/envoy-rev2.json|jq '.node.id')
  fi

  if [ -z "$cluster" ]; then
    cluster=$(kubectl exec --namespace $namespace -it --context $context -c istio-proxy $pod_id -- cat /etc/istio/proxy/envoy-rev2.json|jq '.node.cluster')
  fi

  if [ -z "$node_type" ]; then
    node_type=$(kubectl exec --namespace $namespace -it --context $context -c istio-proxy $pod_id -- cat /etc/istio/proxy/envoy-rev2.json|jq '.node.type')
  fi
fi

if [ "$check_clusters" == "true" ]; then
  curl -X POST \
  http://localhost:9010/ads \
  -H 'Content-Type: application/json' \
  -d '{
	"node": {
		"id": '$node_id',
		"cluster": '$cluster',
		"build_version": "1381673ad2d74bab4667942abdd8ef75c812e75e/1.8.0-dev/Clean/RELEASE"
	},
	"type_url": "type.googleapis.com/envoy.api.v2.Cluster"
}'

  echo "checking clusters for pods $pod_id ..."
  istioctl proxy-config --context $context --namespace $namespace -o $output cluster $pod_id
fi

if [ "$check_endpoints" == "true" ]; then
  curl -X POST \
  http://localhost:9010/ads \
  -H 'Content-Type: application/json' \
  -d '{
	"node": {
		"id": '$node_id',
		"cluster": '$cluster',
		"build_version": "1381673ad2d74bab4667942abdd8ef75c812e75e/1.8.0-dev/Clean/RELEASE"
	},
	"type_url": "type.googleapis.com/envoy.api.v2.ClusterLoadAssignment",
  "resource_names": ['$resource_names']
}'
fi

if [ "$check_listeners" == "true" ]; then
  curl -X POST \
  http://10.29.75.3:9010/ads \
  -H 'Content-Type: application/json' \
  -d '{
	"node": {
		"id": '$node_id',
		"cluster": '$cluster',
		"build_version": "1381673ad2d74bab4667942abdd8ef75c812e75e/1.8.0-dev/Clean/RELEASE"
	},
	"type_url": "type.googleapis.com/envoy.api.v2.Listener"
}'
  echo "checking listeners for pods $pod_id ..."
  istioctl proxy-config --context $context --namespace $namespace -o $output listener $pod_id
fi

if [ "$check_routes" == "true" ]; then
  curl -X POST \
  http://localhost:9010/ads \
  -H 'Content-Type: application/json' \
  -d '{
	"node": {
		"id": '$node_id',
		"cluster": '$cluster',
		"build_version": "1381673ad2d74bab4667942abdd8ef75c812e75e/1.8.0-dev/Clean/RELEASE"
	},
	"type_url": "type.googleapis.com/envoy.api.v2.RouteConfiguration",
	"resource_names": ['$resource_names']
}'
  echo "checking routes for pods $pod_id ..."
  istioctl proxy-config --context $context --namespace $namespace -o $output route $pod_id
fi
set +x
