# Scope As A Service

[![Circle CI](https://circleci.com/gh/weaveworks/service/tree/master.svg?style=shield)](https://circleci.com/gh/weaveworks/service/tree/master) [![Coverage Status](https://coveralls.io/repos/weaveworks/service/badge.svg?branch=coverage&service=github&t=6Kr25T)](https://coveralls.io/github/weaveworks/service?branch=coverage)

![Architecture](docs/architecture.png)

## Run on your laptop

(Assuming its a Mac, and you have a Vagrant Linux VM for development)

Setup a few ssh port forwards:
```
vagrant ssh -- -L1080:smtp.weave.local:1080
vagrant ssh -- -L8080:frontend.weave.local:80
```

Then on you VM, start the service
```
cd <path to service.git>
./run.sh
```

- Go to http://localhost:8080 on you Mac (for the SAAS UI) ans sign up.
- Go to http://localhost:1080 for the mailcatcher UI, you should see a welcome email
- Go to http://localhost:8080/api/users/private/users to approve youself
- Follow link in email (see mailcatcher on http://localhost:1080)
