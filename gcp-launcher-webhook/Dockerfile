FROM alpine:3.6
MAINTAINER Weaveworks Inc <help@weave.works>
WORKDIR /
COPY gcp-launcher-webhook /
COPY ca-certificates.crt /etc/ssl/certs/
EXPOSE 80
ENTRYPOINT ["/gcp-launcher-webhook"]
