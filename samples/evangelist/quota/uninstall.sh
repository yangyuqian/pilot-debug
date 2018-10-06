#!/bin/sh

kubectl delete --namespace istio-system -f samples/evangelist/quota/spec/quotaspec.yaml
kubectl delete --namespace istio-system -f samples/evangelist/quota/spec/quotaspecbinding.yaml
kubectl delete --namespace istio-system -f samples/evangelist/quota/spec/instance.yaml
kubectl delete --namespace istio-system -f samples/evangelist/quota/spec/handler.yaml
kubectl delete --namespace istio-system -f samples/evangelist/quota/spec/rule.yaml
