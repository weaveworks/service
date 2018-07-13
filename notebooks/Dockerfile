FROM alpine:3.6
WORKDIR /
COPY cmd/notebooks/notebooks /
COPY db/migrations /migrations/
EXPOSE 80
ENTRYPOINT ["/notebooks"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="notebooks" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/notebooks" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
