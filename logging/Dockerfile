FROM fluent/fluentd:v0.12.24
USER root
RUN \
    apk add --no-cache --update g++ make ruby-dev && \
    gem install bigdecimal 'fluent-plugin-bigquery: 0.4.4' 'fluent-plugin-prometheus: 0.3.0' && \
    apk del --no-cache g++ make ruby-dev
COPY schema_service_events.json /bigquery/
# fluent.conf is copied by a Dockerfile Build Trigger
COPY fluent-dev.conf /fluentd/etc/

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="logging" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/logging" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
