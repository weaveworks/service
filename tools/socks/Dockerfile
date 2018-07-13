FROM gliderlabs/alpine
WORKDIR /
COPY proxy /
EXPOSE 8000
EXPOSE 8080
ENTRYPOINT ["/proxy"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="authfe" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/authfe" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
