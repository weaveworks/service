FROM alpine:3.6
WORKDIR /
COPY /authfe/authfe /
COPY /authfe/ca-certificates.crt /etc/ssl/certs/
EXPOSE 80
ENTRYPOINT ["/authfe"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="authfe" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/authfe" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
