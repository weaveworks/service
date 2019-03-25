FROM golang:1.10.8
RUN apt-get update && apt-get install -y \
		python-pip \
		python-requests \
		python-yaml \
		file \
		jq \
		shellcheck \
		unzip \
		protobuf-compiler=3.0.0-9 \
		libprotobuf-dev=3.0.0-9 && \
	rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
RUN go clean -i net && \
	go install -tags netgo std && \
	go install -race -tags netgo std sync/atomic
RUN go get -tags netgo \
		github.com/fzipp/gocyclo \
		golang.org/x/lint/golint \
		github.com/kisielk/errcheck \
		github.com/mjibson/esc \
		github.com/client9/misspell/cmd/misspell && \
	rm -rf /go/pkg/ /go/src/
RUN pip install attrs pyhcl yapf==0.16.2 flake8==3.3.0 urllib3==1.23
RUN git clone --branch v0.5 https://github.com/gogo/protobuf /go/src/github.com/gogo/protobuf && \
    cd /go/src/github.com/gogo/protobuf/protoc-gen-gogoslick && \
    go install && \
	rm -rf /go/pkg/ /go/src/
RUN curl -fsSL -o shfmt https://github.com/mvdan/sh/releases/download/v1.3.0/shfmt_v1.3.0_linux_amd64 && \
	echo "b1925c2c405458811f0c227266402cf1868b4de529f114722c2e3a5af4ac7bb2  shfmt" | sha256sum -c && \
	chmod +x shfmt && \
	mv shfmt /usr/bin
RUN mkdir protoc && \
	cd protoc && \
	curl -O -L https://github.com/google/protobuf/releases/download/v3.1.0/protoc-3.1.0-linux-x86_64.zip && \
	echo "7c98f9e8a3d77e49a072861b7a9b18ffb22c98e37d2a80650264661bfaad5b3a  protoc-3.1.0-linux-x86_64.zip" | sha256sum -c && \
	unzip protoc-3.1.0-linux-x86_64.zip && \
	cp bin/protoc /usr/bin/ && \
	chmod o+x /usr/bin/protoc && \
	cd .. && \
	rm -rf protoc
RUN go get github.com/golang/mock/mockgen
COPY build.sh /
ENTRYPOINT ["/build.sh"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="build" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/build" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
