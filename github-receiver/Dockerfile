FROM alpine:3.6
WORKDIR /
COPY github-receiver /
EXPOSE 80
ENTRYPOINT ["/github-receiver"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="github-receiver" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/github-receiver" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
