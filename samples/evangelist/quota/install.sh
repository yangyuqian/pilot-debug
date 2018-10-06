#!/bin/sh

kubectl apply --namespace istio-system -f samples/evangelist/quota/spec/quotaspec.yaml
kubectl apply --namespace istio-system -f samples/evangelist/quota/spec/quotaspecbinding.yaml
kubectl apply --namespace istio-system -f samples/evangelist/quota/spec/instance.yaml
kubectl apply --namespace istio-system -f samples/evangelist/quota/spec/handler.yaml
kubectl apply --namespace istio-system -f samples/evangelist/quota/spec/rule.yaml
