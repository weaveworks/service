FROM fluent/fluentd:v0.12.24
MAINTAINER scope@weave.works
USER root
RUN apk add --no-cache --update g++ make ruby-dev
RUN gem install bigdecimal fluent-plugin-bigquery
RUN apk del --no-cache g++ make ruby-dev
COPY schema_service_events.json /bigquery/
COPY fluent-dev.conf /fluentd/etc/
