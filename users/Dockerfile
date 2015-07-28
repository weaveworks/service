FROM golang:1.4

ADD . /go/src/github.com/weaveworks/service/web
WORKDIR /go/src/github.com/weaveworks/service/web

RUN go get -v -t ./...

RUN go install -v ./...

CMD ["web"]
