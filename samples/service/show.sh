#!/bin/sh

function help() {
  cat << EOF
A script to wrap all examples used in CNUTCon 2018 at Shanghai

Usage:
  sh show.sh ARGS
  
Example:
  sh show.sh --upgrade

Available Arguments:
  --help: help info
  --graceful-seconds: wait a few seconds
  --debug: print debug logs including commands for trouble shooting
  --upgrade: install or upgrade mockserver applications
  --namespace: namespace to install the mockserver, default is soa-test
  --release: release name of the mock service, default mockserver
  --chart: a chart url or local path, default samples/service/charts/mockserver
  --install-example example name to be installed
  --uninstall-example example name to be uninstalled
  --example-root-path example root path
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
      --debug)
        debug=$1
        shift
      ;;
      --upgrade)
        upgrade=$1
        shift
      ;;
      --namespace)
        namespace=$2
        shift
        shift
      ;;
      --release)
        release=$2
        shift
        shift
      ;;
      --chart)
        chart=$2
        shift
        shift
      ;;
      --install-example)
        install_example=$2
        shift
        shift
      ;;
      --uninstall-example)
        uninstall_example=$2
        shift
        shift
      ;;
      --example-root-path)
        example_root_path=$2
        shift
        shift
      ;;
      --graceful-seconds)
        graceful_seconds=$2
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
    help
    exit 1
  fi

  if [ -n "$debug" ]; then
    set -x
  else
    set +x
  fi

  if [ -z "$namespace" ]; then
    kubectl create ns soa-test
    namespace=soa-test
  fi
  # wait a few seconds here, in case of sidecar injector needs some time
  # to get warned up
  kubectl label namespace $namespace istio-injection=enabled
  echo "waiting $graceful_seconds seconds..."
  sleep $graceful_seconds

  if [ -z "$release" ]; then
    release=mockserver
  fi

  if [ -z "$chart" ]; then
    chart=samples/service/charts/mockserver
  fi

  if [ -z "$example_root_path" ]; then
    example_root_path="samples/evangelist"
  fi

  if [ -z "$graceful_seconds" ]; then
    graceful_seconds=10
  fi
}

function restart_deployment() {
  deployment_name=$1
  if [ -z "$deployment_name" ]; then
    echo "deployment_name is required!"
    exit 1
  fi
  deployment_namespace=$2
  if [ -z "$deployment_namespace" ]; then
    deployment_namespace=istio-system
  fi

  kubectl scale --replicas=0 --namespace $deployment_namespace deployments/${deployment_name}
  kubectl scale --replicas=1 --namespace $deployment_namespace deployments/${deployment_name}
}

parse_args $@

# upgrade or install mockserver, and cleanup all monitor data
if [ -n "$upgrade" ]; then
  restart_deployment "istio-telemetry"
  restart_deployment "prometheus"
  restart_deployment "servicegraph"
  restart_deployment "grafana"
  restart_deployment "istio-tracing"

  helm upgrade --install --timeout 1200 --wait --namespace $namespace $release $chart
  echo "waiting $graceful_seconds seconds..."
  sleep $graceful_seconds
fi

if [ -n "$install_example" ]; then
  if [ -f "${example_root_path}/${install_example}/install.sh" ]; then
    sh ${example_root_path}/${install_example}/install.sh
  else
    kubectl apply --namespace $namespace -f ${example_root_path}/${install_example}/spec/
  fi
  echo "waiting $graceful_seconds seconds..."
  sleep $graceful_seconds
fi

if [ -n "$uninstall_example" ]; then
  if [ -f "${example_root_path}/${uninstall_example}/uninstall.sh" ]; then
    sh ${example_root_path}/${uninstall_example}/uninstall.sh
  else
    kubectl delete --namespace $namespace -f ${example_root_path}/${uninstall_example}/spec/
  fi
  echo "waiting $graceful_seconds seconds..."
  sleep $graceful_seconds
fi

