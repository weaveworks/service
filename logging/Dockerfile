FROM fluent/fluentd:v1.2.4
RUN \
    apk add --no-cache --update g++ make ruby-dev && \
    gem install bigdecimal 'fluent-plugin-bigquery: 2.0.0.beta' 'fluent-plugin-prometheus: 1.0.1' && \
    apk del --no-cache g++ make ruby-dev
COPY /logging/schema_service_events.json /bigquery/
COPY /logging/fluent.conf /fluentd/etc/
COPY /logging/fluent-dev.conf /fluentd/etc/

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="logging" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/logging" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
