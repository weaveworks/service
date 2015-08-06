# Scope As A Service

[![Circle CI](https://circleci.com/gh/weaveworks/service/tree/master.svg?style=shield)](https://circleci.com/gh/weaveworks/service/tree/master) [![Coverage Status](https://coveralls.io/repos/weaveworks/service/badge.svg?branch=coverage&service=github&t=6Kr25T)](https://coveralls.io/github/weaveworks/service?branch=coverage)

![Architecture](docs/architecture.png)

## Run on your laptop

(Assuming its a Mac, and you have a Vagrant Linux VM for development)

On you VM, start the services (make sure you ```eval $(weave env)```)
```
cd <path to service.git>
./run.sh
```

Then on you mac, start the proxy:
```
vagrant ssh-config >>~/.ssh/config # setup ssh for your vagrant VM
./connect.sh <hostname>
```

And configure you proxy settings for http://localhost:8080/proxy.pac

## Test workflow

- Go to http://run.weave.works on you Mac (for the SAAS UI) and sign up.
- Go to http://smtp.weave.local:1080 for the mailcatcher UI, you should see a welcome email
- Go to http://users.weave.local/private/api/users to approve youself
- Follow link in email (see mailcatcher on http://smtp.weave.local:1080)
