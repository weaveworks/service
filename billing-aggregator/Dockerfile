FROM alpine:3.6
EXPOSE 80
ENTRYPOINT ["/aggregator"]
RUN apk add --update \
    ca-certificates \
  && rm -rf /var/cache/apk/*
ADD /billing-aggregator/migrations /migrations
ADD /billing-aggregator/aggregator /

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="billing-aggregator" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/billing-aggregator" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
