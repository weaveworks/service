FROM alpine:3.6
WORKDIR /
COPY metrics /
ENTRYPOINT ["/metrics"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="metrics" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/metrics" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
