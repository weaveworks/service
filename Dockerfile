FROM golang:1.4

RUN apt-get update -y && apt-get -y install postgresql-client

ADD . /go/src/github.com/weaveworks/service
WORKDIR /go/src/github.com/weaveworks/service

RUN make deps

CMD ["/bin/bash"]
