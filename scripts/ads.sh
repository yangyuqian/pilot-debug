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
  --node-id: node id

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
      --type)
        type=$2
        shift
        shift
      ;;
      --type-url)
        type_url=$2
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

if [ "$upgrade" == "true" ]; then
  go get -u github.com/yangyuqian/pilot-debug
fi

if [ -z "$pilot_target" ]; then
  pilot_target=$PILOT_ADDR
fi

if [ "$debug_server" == "true" ]; then
  pilot-debug --target $pilot_target
fi

if [ "$check_clusters" == "true" ]; then
  curl -X POST \
  http://localhost:9010/ads \
  -H 'Content-Type: application/json' \
  -d '{
	"node": {
		"id": "'$node_id'",
		"cluster": "'$cluster'",
		"build_version": "1381673ad2d74bab4667942abdd8ef75c812e75e/1.8.0-dev/Clean/RELEASE",
		"type": "'$type'"
	},
	"type_url": "'$type_url'"
}'
fi
