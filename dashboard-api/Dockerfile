FROM alpine:3.7
MAINTAINER Weaveworks Inc <help@weave.works>
LABEL org.label-schema.name="dashboard-api" \
        org.label-schema.description="serve service dashboards" \
        org.label-schema.vcs-url="https://github.com/weaveworks/service/tree/master/dashboard-service" \
        org.label-schema.vendor="Weaveworks" \
        org.label-schema.schema-version="1.0"

WORKDIR /
COPY dashboard-api /
EXPOSE 80
EXPOSE 4772
ENTRYPOINT ["/dashboard-api"]
