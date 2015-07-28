FROM golang:1.4

ADD . /go/src/github.com/weaveworks/service/users
WORKDIR /go/src/github.com/weaveworks/service/users

RUN go get -v -t ./...

RUN go install -v ./...

CMD ["users"]
