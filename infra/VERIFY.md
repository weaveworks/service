# Verify a Kubernetes cluster

We will work in the helloworld directory.

```
$ cd helloworld
```

Tell Kubernetes to download and run a simple web server.
Create a new replication controller, from the file.

```
$ kubectl create -f helloworld-rc.yaml
```

Check that it was created.

```
$ kubectl get rc
CONTROLLER         CONTAINER(S)   IMAGE(S)   SELECTOR   REPLICAS
helloworld-1.0.0   helloworld     . . .      . . .      1
```

Check that a pod is running.

```
$ kubectl get pods
NAME                     READY     STATUS    RESTARTS   AGE
helloworld-1.0.0-uxnyk   1/1       Running   0          1m
```

Use kubectl to forward a port from your local machine to the pod directly.
Then, curl that port to see your pod is working.
Note you need to copy/paste the specific pod name from the above step.

```
$ kubectl port-forward -p helloworld-1.0.0-uxnyk 10000:80
$ curl -Ss -XGET localhost:10000
Hello world
```

Scale the number of replicas up to 3.

```
$ kubectl scale --replicas=3 rc helloworld-1.0.0
scaled
```

Verify.

```
$ kubectl get rc
CONTROLLER         CONTAINER(S)   IMAGE(S)   SELECTOR   REPLICAS
helloworld-1.0.0   helloworld     ...        ...        3
```

To expose this application to the world, we need to create a Kubernetes service.
A service bridges a set of pods (matching some label query) and a load balancer endpoint.
Kubernetes automatically uses the load balancer of the underlying platform; in our case, an ELB.
Our helloworld service will match all app=helloworld pods, ignoring all other label dimensions like version.

```
$ kubectl create -f helloworld-svc.yaml
```

Inspect the service until you see the ELB endpoint that was created.
It may take several minutes to appear in the output.

```
$ kubectl describe svc helloworld
Name:                   helloworld
Namespace:              default
Labels:                 app=helloworld
Selector:               app=helloworld
Type:                   LoadBalancer
IP:                     10.0.254.122
LoadBalancer Ingress:   ab1896c8f7eff11e58b1502f93cffe5e-1066700612.us-west-2.elb.amazonaws.com
Port:                   <unnamed>       80/TCP
NodePort:               <unnamed>       30088/TCP
Endpoints:              10.244.3.15:80
Session Affinity:       None
No events.
```

In another terminal, set up a loop to continuously GET the ELB.
We'll use that to verify the version upgrade works as expected.
It may take several minutes for the ELB to work correctly.

```
$ bash -c 'while true; do curl -Ss -XGET ab1896c8f7eff11e58b1502f93cffe5e-1066700612.us-west-2.elb.amazonaws.com; sleep 0.5; done'
Hello world
Hello world
Hello world
```

Now, we'll deploy a new version of our application, which prints "Foo bar" instead of "Hello world".
Modify the replication controller to control the 2.0.0 container.

```
$ sed -i'.bak' 's/1.0.0/2.0.0/g' helloworld-rc.yaml ; rm -f *.bak
```

Now, let's do a rolling update, from 1.0.0 to 2.0.0.
We'll wait 3s between starting a new pod and killing an old one.
In production, you want to wait longer, 1m or more.

```
$ kubectl rolling-update helloworld-1.0.0 -f helloworld-rc.yaml --update-period=3s
```

In your other terminal, you should see "Hello world" and "Foo bar" interleaved, and then only "Foo bar".
All done.
Now, let's tear everything down.

```
$ kubectl delete svc helloworld
$ kubectl delete rc helloworld-2.0.0
$ git checkout -- helloworld-rc.yaml
```

No pods left.

```
$ kubectl get pods
NAME      READY     STATUS    RESTARTS   AGE
```
