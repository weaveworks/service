FROM alpine:3.6
EXPOSE 80
ENTRYPOINT ["/aggregator"]
RUN apk add --update \
    ca-certificates \
  && rm -rf /var/cache/apk/*
ADD migrations /migrations
ADD aggregator /

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="billing-aggregator" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/main/billing-aggregator" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
