FROM golang:1.17-bullseye
RUN apt-get update && \
    apt-get install -y \
      sudo \
      curl \
      file \
      git \
      jq \
      libprotobuf-dev \
      make \
      protobuf-compiler \
      python3 \
      python3-pip \
      python3-requests \
      python3-yaml \
      libssl-dev \
      python3-openssl \
      shellcheck \
      unzip && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

ENV GO111MODULE off

RUN mkdir protoc && \
    cd protoc && \
    curl -O -L https://github.com/protocolbuffers/protobuf/releases/download/v3.18.1/protoc-3.18.1-linux-x86_64.zip && \
    unzip protoc-3.18.1-linux-x86_64.zip && \
    cp bin/protoc /usr/bin/ && \
    chmod o+x /usr/bin/protoc && \
    cd .. && \
    rm -rf protoc
RUN go clean -i net && \
    go get -tags netgo std && \
    go get -race -tags netgo std
RUN go get -tags netgo \
        github.com/FiloSottile/gvt \
        github.com/client9/misspell/cmd/misspell \
        github.com/fzipp/gocyclo/cmd/gocyclo \
        github.com/gogo/protobuf/protoc-gen-gogoslick \
        golang.org/x/lint/golint \
        github.com/kisielk/errcheck \
        github.com/mjibson/esc && \
    rm -rf /go/pkg /go/src

RUN go get github.com/golang/mock/mockgen
COPY build.sh /
ENTRYPOINT ["/build.sh"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="build" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/build" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
