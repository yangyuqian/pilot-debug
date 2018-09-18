#!/bin/sh
checktime=60

function wt(){
  echo "wait $checktime seconds ..."
  sleep $checktime
}

soa_ns=soa-test

echo "installing istio ..."
kubectl apply -f istio-full.yaml


wt
echo "installing bookinfo example ..."
kubectl apply --namespace $soa_ns -f samples/bookinfo.yaml


wt
echo "installing destinationrule and virtual services ..."
kubectl apply --namespace $soa_ns -f samples/networking/destination-rule-all-mtls.yaml


wt
echo "installing HTTP/1.1 gateway ..."
kubectl apply --namespace $soa_ns -f samples/networking/bookinfo-gateway.yaml

wt
echo "installing HTTPS gateway ..."
kubectl apply --namespace $soa_ns -f samples/networking/bookinfo-gateway-tls.yaml



