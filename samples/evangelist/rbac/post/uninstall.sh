#!/bin/sh

namespace=soa-test

kubectl delete --namespace ${namespace} -f samples/evangelist/rbac/post/spec/spec.yaml
sleep 10
kubectl delete --namespace ${namespace} -f samples/evangelist/rbac/post/spec/init.yaml
