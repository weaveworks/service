FROM alpine:3.6
EXPOSE 80
ENTRYPOINT ["/metrics-usage"]
ADD metrics-usage /

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="metrics-usage" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/metrics-usage" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
