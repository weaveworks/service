## Kubernetes Manifest Generator

### Usage examples

When user wants to expose the app via `NodePort` mechanism (works in any Kubernetes cluster):
```
kubectl create -f 'https://scope.weave.works/launch/k8s/weavescope.json?k8s-service-type=NodePort' --validate=false
```

Most basic way, lets user expose the app port manually:
```
kubectl create -f 'https://scope.weave.works/launch/k8s/weavescope.json' --validate=false
```

When user wants to user the service:
```
kubectl create -f 'https://scope.weave.works/launch/k8s/weavescope.json?v=v0.13.1&service-token=b4wdto5ifepb6ggq4eb384gi46biyews' --validate=false
```

When user wants to test image with tag `test-issue123` and use `LoadBalancer` to expose the app (only works in some clouds):
```
kubectl create -f 'https://scope.weave.works/launch/k8s/weavescope.json?k8s-service-type=LoadBalancer&v=test-issue123'
```
