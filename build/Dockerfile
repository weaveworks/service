FROM weaveworks/build-golang:1.13.5-stretch
RUN go get github.com/golang/mock/mockgen
COPY build.sh /
ENTRYPOINT ["/build.sh"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="build" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/build" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
