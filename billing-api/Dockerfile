FROM alpine:3.6
EXPOSE 80
ENTRYPOINT ["/api"]
RUN apk add --update \
    ca-certificates \
  && rm -rf /var/cache/apk/*
ADD /billing-api/db/migrations /migrations
ADD /billing-api/api /

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="billing-api" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/billing-api" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
