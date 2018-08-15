FROM alpine:3.6
EXPOSE 80
ENTRYPOINT ["/uploader"]
RUN apk add --update \
    ca-certificates \
  && rm -rf /var/cache/apk/*
ADD /billing-uploader/migrations /migrations
ADD /billing-uploader/uploader /

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="billing-uploader" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/billing-uploader" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
