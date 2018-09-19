FROM alpine:3.6
WORKDIR /
COPY templates/*.html /templates/
COPY templates/*.text /templates/
COPY db/migrations /migrations/
COPY ca-certificates.crt /etc/ssl/certs/
COPY cmd/users/users /
EXPOSE 80
ENTRYPOINT ["/users"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="users" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/users" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
