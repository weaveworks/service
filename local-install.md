# Running Weave Cloud locally in plain Kubernetes

I was unable to use Minikube, since my home machine runs Windows and I
use VirtualBox.  So I ran it up inside vanilla Kubernetes, installed
via `kubeadm`.

## Creating the host

We have a [Vagrant configuration](https://github.com/weaveworks/weave/blob/master/test/Vagrantfile)
inside Weave Net; I took that and

* Bumped RAM to 6G
* Bumped CPUs to 2

## Installing Kubernetes

Run instructions at http://kubernetes.io/docs/getting-started-guides/kubeadm/

    apt-get update && apt-get install -y apt-transport-https
    curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
    cat <<EOF > /etc/apt/sources.list.d/kubernetes.list
    deb http://apt.kubernetes.io/ kubernetes-xenial main
    EOF
    apt-get update
    # Don't need to install Docker because our base install does that already
    apt-get install -y kubelet kubeadm kubectl kubernetes-cni

Then I fire up Kubernetes, using an extra flag to stop it taking Vagrant's default NAT interface:

    kubeadm init --api-advertise-addresses=192.168.48.11

We need to get some credentials to log in on the controlling machine.

    ssh vagrant@192.168.48.11 sudo cat /etc/kubernetes/admin.conf > ~/.kube/config

Now we can see what's running:

    kubectl get pods -o wide --all-namespaces
    NAMESPACE     NAME                              READY     STATUS              RESTARTS   AGE
    kube-system   dummy-2088944543-wv96b            1/1       Running             0          16m
    kube-system   etcd-host1                        1/1       Running             0          15m
    kube-system   kube-apiserver-host1              1/1       Running             2          16m
    kube-system   kube-controller-manager-host1     1/1       Running             0          15m
    kube-system   kube-discovery-1769846148-3m7zp   1/1       Running             0          16m
    kube-system   kube-dns-2924299975-3l1wb         0/4       ContainerCreating   0          16m
    kube-system   kube-proxy-7d1b4                  1/1       Running             0          16m
    kube-system   kube-scheduler-host1              1/1       Running             0          15m

No kube-dns yet because there's no pod network.  Let's use Weave Net!

    $ kubectl apply -f https://git.io/weave-kube
    $ kubectl get pods -o wide --all-namespaces
    NAMESPACE     NAME                              READY     STATUS    RESTARTS   AGE       IP              NODE
    kube-system   dummy-2088944543-wv96b            1/1       Running   0          18m       192.168.48.11   host1
    kube-system   etcd-host1                        1/1       Running   0          16m       192.168.48.11   host1
    kube-system   kube-apiserver-host1              1/1       Running   2          18m       192.168.48.11   host1
    kube-system   kube-controller-manager-host1     1/1       Running   0          17m       192.168.48.11   host1
    kube-system   kube-discovery-1769846148-3m7zp   1/1       Running   0          18m       192.168.48.11   host1
    kube-system   kube-dns-2924299975-3l1wb         4/4       Running   0          17m       10.32.0.2       host1
    kube-system   kube-proxy-7d1b4                  1/1       Running   0          17m       192.168.48.11   host1
    kube-system   kube-scheduler-host1              1/1       Running   0          17m       192.168.48.11   host1
    kube-system   weave-net-0b183                   2/2       Running   0          33s       192.168.48.11   host1

Cool!

Now, since we only have one host, we need to remove the restriction
that kubeadm places on running on master:

    kubectl get node host1 -o yaml | sed -e "s/taints: .*master.*NoSchedule.*$/taints: '[]'/" | kubectl apply -f -

## Installing Weave Cloud software

The authfe and users images aren't all on quay, so you'll need to
build them, then copy over to the Kubernetes host.

    cd github.com/weaveworks/service
    make
    docker save quay.io/weaveworks/authfe:latest | ssh 192.168.48.11 docker load 
    docker save quay.io/weaveworks/users:latest | ssh 192.168.48.11 docker load
    docker save quay.io/weaveworks/logging:latest | ssh 192.168.48.11 docker load

(The address 192.168.48.11 comes from the Vagrantfile config in Weave Net)

Now start up the system, as per the service-conf instructions:

    kubectl apply -f k8s/local
    kubectl apply -f k8s/local/default

[plus any more you need to run]

Now you can access the UI at http://192.168.48.11:30081

