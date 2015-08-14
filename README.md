# Scope As A Service

[![Circle CI](https://circleci.com/gh/weaveworks/service/tree/master.svg?style=shield)](https://circleci.com/gh/weaveworks/service/tree/master) [![Coverage Status](https://coveralls.io/repos/weaveworks/service/badge.svg?branch=coverage&service=github&t=6Kr25T)](https://coveralls.io/github/weaveworks/service?branch=coverage)

![Architecture](docs/architecture.png)

## Run

On your Linux host or VM, start Weave. You must build Weave yourself from
current master, using Weave 1.0.1 or latest on Docker Hub is not recent enough!
Also, you must make sure running weave globally will invoke the weave script
from current master.

```
cd $GOPATH/src/github.com/weaveworks
git clone https://github.com/weaveworks/weave
cd weave
make
sudo rm -f /usr/local/bin/weave
sudo ln -s $GOPATH/src/github.com/weaveworks/weave/weave /usr/local/bin/weave
weave launch
eval $(weave env)
```

Now, still on your Linux host or VM, launch the run script.

```
cd $GOPATH/src/github.com/weaveworks/service
./run.sh
```

Finally, on your Mac, start the proxy. If you're using a Vagrant VM, you can
use the connect.sh script.

```
vagrant ssh-config >> ~/.ssh/config
./connect.sh <hostname>
```

When configuring your system proxies, ensure that proxies are *not*
bypassed for *.local.

## Test workflow

From your Mac,

1. http://scope.weave.works — sign up
1. http://smtp.weave.local — you should see a welcome email
1. http://users.weave.local/private/api/users — approve yourself
1. http://smtp.weave.local — click on the link in the approval email
1. Use the token in the approval email to start a probe:

  ./scope launch --service-token=lhFr_M4SwtOmjLrrxHc2 frontend.weave.local:80

1. Navigate to http://scope.weave.works and behold the beauty

Note:

- You'll need to preload a recent build of the scope image

