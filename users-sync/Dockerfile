FROM alpine:3.6
WORKDIR /
COPY cmd/users-sync /
EXPOSE 80
ENTRYPOINT ["/users-sync"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="users-sync" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/users-sync" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
