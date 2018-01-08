FROM alpine:3.7
MAINTAINER Weaveworks Inc <help@weave.works>
LABEL org.label-schema.name="gcp-service" \
        org.label-schema.description="service to interact with GCP's APIs" \
        org.label-schema.vcs-url="https://github.com/weaveworks/service/tree/master/gcp-service" \
        org.label-schema.vendor="Weaveworks" \
        org.label-schema.schema-version="1.0"

WORKDIR /
COPY gcp-service /
EXPOSE 80
EXPOSE 4772
ENTRYPOINT ["/gcp-service"]
