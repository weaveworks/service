FROM alpine:3.6
RUN apk add --no-cache ca-certificates
WORKDIR /
COPY cmd/eventmanager/eventmanager /
COPY       db/migrations /migrations
COPY templates /templates
EXPOSE 80
ENTRYPOINT ["/eventmanager"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="notification-eventmanager" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/notification-eventmanager" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
