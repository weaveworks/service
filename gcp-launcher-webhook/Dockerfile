FROM alpine:3.14
WORKDIR /
COPY gcp-launcher-webhook /
EXPOSE 80
ENTRYPOINT ["/gcp-launcher-webhook"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="gcp-launcher-webhook" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/main/gcp-launcher-webhook" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
