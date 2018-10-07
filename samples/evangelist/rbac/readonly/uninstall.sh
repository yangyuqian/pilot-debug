#!/bin/sh

namespace=soa-test

kubectl delete --namespace ${namespace} -f samples/evangelist/rbac/readonly/spec/spec.yaml
sleep 10
kubectl delete --namespace ${namespace} -f samples/evangelist/rbac/readonly/spec/init.yaml
