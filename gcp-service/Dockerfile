FROM alpine:3.7
RUN apk add --no-cache -q ca-certificates
WORKDIR /
COPY gcp-service /
EXPOSE 80
EXPOSE 4772
ENTRYPOINT ["/gcp-service"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="gcp-service" \
      org.opencontainers.image.description="service to interact with GCP's APIs" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/gcp-service" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
