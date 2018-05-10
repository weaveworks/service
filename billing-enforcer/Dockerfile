FROM alpine:3.6
EXPOSE 80
ENTRYPOINT ["/enforcer"]
RUN apk add --update \
    ca-certificates \
  && rm -rf /var/cache/apk/*
ADD enforcer /

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="billing-enforcer" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/billing-enforcer" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
