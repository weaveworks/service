FROM alpine:3.14
WORKDIR /
COPY authfe /
EXPOSE 80
ENTRYPOINT ["/authfe"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="authfe" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/main/authfe" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
