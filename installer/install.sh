#!/bin/sh
checktime=60
soa_ns=soa-test

function wt(){
  echo "wait $checktime seconds ..."
  sleep $checktime
}

function reinstall(){
  echo "cleanup $1 ..."
  kubectl delete --namespace $soa_ns -f $1

  wt
  echo "installing $1 ..."
  kubectl apply --namespace $soa_ns -f $1
}

reinstall "istio-full.yaml"
reinstall "samples/networking/soa-gateway.yaml"
reinstall "samples/networking/virtualservice-destinationrule-grafana.yaml"
reinstall "samples/networking/virtualservice-destinationrule-servicegraph.yaml"
reinstall "samples/networking/virtualservice-destinationrule-tracing.yaml"
