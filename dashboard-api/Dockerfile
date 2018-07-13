FROM alpine:3.7
WORKDIR /
COPY dashboard-api /
EXPOSE 80
EXPOSE 4772
ENTRYPOINT ["/dashboard-api"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="dashboard-api" \
      org.opencontainers.image.description="serve service dashboards" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/dashboard-api" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
