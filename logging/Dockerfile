FROM fluent/fluentd:v0.12.24
MAINTAINER scope@weave.works
USER root
RUN \
    apk add --no-cache --update g++ make ruby-dev && \
    gem install bigdecimal 'fluent-plugin-bigquery: 0.4.4' 'fluent-plugin-prometheus: 0.3.0' && \
    apk del --no-cache g++ make ruby-dev
