FROM alpine:3.4
MAINTAINER Weaveworks Inc <help@weave.works>
WORKDIR /
COPY pr-assigner /
# work around https://github.com/golang/go/issues/15749
COPY ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/pr-assigner"]
