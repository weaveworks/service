FROM alpine:3.7

MAINTAINER Weaveworks Inc <help@weave.works>
LABEL org.label-schema.name="flux-api" \
      org.label-schema.description="service to provide Deploy (Flux) API" \
      org.label-schema.vcs-url="https://github.com/weaveworks/service/tree/master/flux-api" \
      org.label-schema.vendor="Weaveworks" \
      org.label-schema.schema-version="1.0"

WORKDIR /home/flux
ENTRYPOINT [ "/sbin/tini", "--", "flux-api" ]
RUN apk add --no-cache ca-certificates tini
ADD migrations.tar /home/flux
COPY flux-api /usr/local/bin/
