FROM fluent/fluentd:v1.12.2-1.0

# Use root account to use apk
USER root

RUN \
    apk add --no-cache --update --virtual .build-deps \
        build-base ruby-dev \
 && gem install bigdecimal 'prometheus-client: 2.1.0' 'fluent-plugin-bigquery: 2.2.0' 'fluent-plugin-prometheus: 2.0.0' \
 && gem sources --clear-all \
 && apk del .build-deps \
 && rm -rf /tmp/* /var/tmp/* /usr/lib/ruby/gems/*/cache/*.gem
COPY schema_events.json /bigquery/
COPY fluent-dev.conf /fluentd/etc/
COPY fluent.conf /fluentd/etc/

USER fluent

EXPOSE 24225
EXPOSE 24231

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="billing-ingester" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/billing-ingester" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
