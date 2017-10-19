FROM fluent/fluentd:v0.12.40
MAINTAINER dev@weave.works
USER root

# Install fluent-plugin-prometheus for fluentd 0.12
RUN gem install 'fluent-plugin-prometheus: < 1.0.0'

# Install fluent-plugin-bigquery for fluentd 0.12
RUN \
    apk add --no-cache --update g++ make ruby-dev && \
    gem install bigdecimal 'fluent-plugin-bigquery: < 1.0.0' && \
    apk del --no-cache g++ make ruby-dev

# Cleanup
WORKDIR /home/fluent
COPY schema_events.json /bigquery/
COPY fluent-dev.conf /fluentd/etc/
COPY fluent.conf /fluentd/etc/
EXPOSE 24225
EXPOSE 24231
