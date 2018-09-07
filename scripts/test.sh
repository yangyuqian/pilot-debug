#!/bin/bash

kubectl get pods --namespace kube-system --context ui.od.k8s.local

###################################################################################################################################################################
ISTIO_VERSION=1.0.1
ISTIO_TEST_NS=istio-test

v1_pod_id=`kubectl get pods --context ui.od.k8s.local --namespace ${ISTIO_TEST_NS} --selector="app=helloworld,version=v1" -o jsonpath='{ .items[0].metadata.name }'`
v2_pod_id=`kubectl get pods --context ui.od.k8s.local --namespace ${ISTIO_TEST_NS} --selector="app=helloworld,version=v2" -o jsonpath='{ .items[0].metadata.name }'`

set -x
kubectl logs --context ui.od.k8s.local --namespace ${ISTIO_TEST_NS} -c istio-proxy $v1_pod_id
kubectl logs --context ui.od.k8s.local --namespace ${ISTIO_TEST_NS} -c istio-proxy $v2_pod_id
kubectl exec -it --context ui.od.k8s.local --namespace ${ISTIO_TEST_NS} -c istio-proxy $v1_pod_id -- curl -i istio-pilot.istio-system:15010
kubectl exec -it --context ui.od.k8s.local --namespace ${ISTIO_TEST_NS} -c istio-proxy $v2_pod_id -- curl -i istio-pilot.istio-system:15010
