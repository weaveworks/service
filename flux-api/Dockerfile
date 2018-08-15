FROM alpine:3.7
WORKDIR /home/flux
ENTRYPOINT [ "/sbin/tini", "--", "flux-api" ]
RUN apk add --no-cache ca-certificates tini
ADD /flux-api/migrations.tar /home/flux
COPY /flux-api/flux-api /usr/local/bin/

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="flux-api" \
      org.opencontainers.image.description="service to provide Deploy (Flux) API" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/flux-api" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
