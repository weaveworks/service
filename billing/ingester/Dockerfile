FROM fluent/fluentd:v0.12.40
MAINTAINER dev@weave.works
USER root
RUN apk update
RUN apk add --no-cache --update g++ make ruby-dev git

# Install fluent-plugin-prometheus
RUN gem install fluent-plugin-prometheus:0.3.0

# Install our forked bigquery fluent plugin
RUN git clone --depth 1 -b 0.4.0-late_timestamp https://github.com/weaveworks/fluent-plugin-bigquery.git
WORKDIR fluent-plugin-bigquery
RUN gem install bundler:1.15.4 io-console:0.4.6 bigdecimal:1.3.2
RUN bundle
RUN gem build fluent-plugin-bigquery.gemspec
RUN gem install fluent-plugin-bigquery-0.4.0.gem

# Cleanup
WORKDIR /home/fluent
RUN apk del --no-cache g++ make ruby-dev git
COPY schema_events.json /bigquery/
COPY fluent-dev.conf /fluentd/etc/
COPY fluent.conf /fluentd/etc/
EXPOSE 24225
EXPOSE 24231
