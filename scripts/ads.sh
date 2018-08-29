#!/bin/sh

function help() {
  cat << EOF
Check aDS settings via calling async gRPC endpoints of Pilot.

Usage:
  sh ads.sh [ARGS]

Example:

Available Arguments:
  --upgrade: upgrade pilot-debug server, calling gRPC endpoints directly to simulate xDS discovery with pilot
  --debug-server: start debug server connected with pilot server
  --pilot-target: pilot target address

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


