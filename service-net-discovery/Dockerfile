FROM       alpine:3.6
RUN        apk add --no-cache ca-certificates
COPY       cmd/discovery/discovery /bin/discovery
EXPOSE     80
ENTRYPOINT [ "/bin/discovery" ]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="service-net-discovery" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/main/service-net-discovery" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
