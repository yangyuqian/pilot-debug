#!/bin/sh
checktime=60
soa_ns=od-system

function wt(){
  echo "wait $checktime seconds ..."
  sleep $checktime
}

function reinstall(){
  ns=$soa_ns
  if [ -n "$2" ]; then
    ns=$2
  fi

  echo "initialize namespace $ns ..."
  wt
  kubectl create ns $ns

  echo "cleanup $1 ..."
  wt
  kubectl delete --namespace $ns -f $1 || echo "removed $1 ..."

  echo "installing $1 ..."
  wt
  kubectl apply --namespace $ns -f $1
}

reinstall "istio-full.yaml" "istio-system"
# reinstall "istio-no-crd.yaml" "istio-system"
reinstall "samples/networking/soa-gateway.yaml"
# reinstall "samples/networking/virtualservice-destinationrule-solr.yaml"
reinstall "samples/networking/virtualservice-destinationrule-grafana.yaml"
reinstall "samples/networking/virtualservice-destinationrule-servicegraph.yaml"
reinstall "samples/networking/virtualservice-destinationrule-tracing.yaml"
# reinstall "samples/networking/virtualservice-destinationrule-pilot.yaml"
