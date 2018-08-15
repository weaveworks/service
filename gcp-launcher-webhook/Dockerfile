FROM alpine:3.6
WORKDIR /
COPY gcp-launcher-webhook /
COPY gcp-launcher-webhook/ca-certificates.crt /etc/ssl/certs/
EXPOSE 80
ENTRYPOINT ["/gcp-launcher-webhook"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="gcp-launcher-webhook" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/gcp-launcher-webhook" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
