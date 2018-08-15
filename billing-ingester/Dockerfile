FROM fluent/fluentd:v0.12.40

USER root

# Install fluent-plugin-prometheus for fluentd 0.12
RUN gem install 'fluent-plugin-prometheus: < 1.0.0'

# Install fluent-plugin-bigquery for fluentd 0.12
RUN \
    apk add --no-cache --update g++ make ruby-dev && \
    gem install bigdecimal 'fluent-plugin-bigquery: < 1.0.0' && \
    apk del --no-cache g++ make ruby-dev

# Install & inject rbtrace so that we can debug memory leaks in a running process!
RUN \
    apk add --no-cache g++ make ruby-dev autoconf automake libtool && \
    gem install rbtrace && \
    sed -i -E "/^load /i require 'rbtrace'" /usr/bin/fluentd && \
    apk del --no-cache g++ make ruby-dev autoconf automake libtool

# Cleanup
WORKDIR /home/fluent
COPY /billing-ingester/schema_events.json /bigquery/
COPY /billing-ingester/fluent-dev.conf /fluentd/etc/
COPY /billing-ingester/fluent.conf /fluentd/etc/
EXPOSE 24225
EXPOSE 24231

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="billing-ingester" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/billing-ingester" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
