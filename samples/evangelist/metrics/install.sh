#!/bin/sh

kubectl apply --namespace istio-system -f samples/evangelist/metrics/spec/handler.yaml
kubectl apply --namespace istio-system -f samples/evangelist/metrics/spec/rule-new.yaml
