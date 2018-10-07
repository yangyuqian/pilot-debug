#!/bin/sh

namespace=soa-test

kubectl apply --namespace ${namespace} -f samples/evangelist/rbac/readonly/spec/init.yaml
sleep 10
kubectl apply --namespace ${namespace} -f samples/evangelist/rbac/readonly/spec/spec.yaml
