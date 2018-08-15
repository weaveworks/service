FROM alpine:3.6
WORKDIR /
COPY /users/cmd/users/users /
COPY /common/templates /templates/
COPY /users/templates/*.html /templates/
COPY /users/templates/*.text /templates/
COPY /users/db/migrations /migrations/
COPY /users/ca-certificates.crt /etc/ssl/certs/
EXPOSE 80
ENTRYPOINT ["/users"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="users" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/users" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
