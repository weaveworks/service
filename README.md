# Scope As A Service

[![Circle CI](https://circleci.com/gh/weaveworks/service/tree/master.svg?style=shield)](https://circleci.com/gh/weaveworks/service/tree/master) [![Coverage Status](https://coveralls.io/repos/weaveworks/service/badge.svg?branch=coverage&service=github&t=6Kr25T)](https://coveralls.io/github/weaveworks/service?branch=coverage)

![Architecture](docs/architecture.png)

## Run on your laptop

(Assuming its a Mac, and you have a Vagrant Linux VM for development)

```
vagrant ssh -- -L8080:frontend.weave.local:80
cd <path to service.git>
./run.sh
```

Then go to localhost:8080 on you Mac.
