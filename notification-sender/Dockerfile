FROM alpine:3.6

RUN apk add --no-cache ca-certificates
WORKDIR /
COPY /notification-sender/cmd/sender/sender /
EXPOSE 80
ENTRYPOINT ["/sender"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="notification-sender" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/notification-sender" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
