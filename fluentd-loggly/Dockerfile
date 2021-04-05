FROM fluent/fluentd:v1.11.5-debian-1.0
USER root
RUN buildDeps="sudo make gcc g++ libc-dev autoconf automake libtool libltdl-dev" \
 && apt-get update \
 && apt-get install -y --no-install-recommends $buildDeps \
 && sudo gem install \
         json:2.1.0 \
         fluent-plugin-kubernetes_metadata_filter:2.5.0 \
         fluent-plugin-cloudwatch-logs:0.4.4 \
         fluent-plugin-systemd:1.0.1 \
         fluent-plugin-concat:2.2.0 \
         fluent-plugin-fields-parser:0.1.2 \
         fluent-plugin-rewrite-tag-filter:2.3.0 \
         fluent-plugin-elasticsearch:4.2.2 \
         prometheus-client:2.1.0 \
         fluent-plugin-prometheus:2.0.0 \
 && sudo gem sources --clear-all \
 && SUDO_FORCE_REMOVE=yes \
    apt-get purge -y --auto-remove \
                  -o APT::AutoRemove::RecommendsImportant=false \
                  $buildDeps \
 && rm -rf /var/lib/apt/lists/* \
 && rm -rf /tmp/* /var/tmp/* /usr/lib/ruby/gems/*/cache/*.gem

# Port for Prometheus metrics
EXPOSE 24231

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="fluentd-loggly" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/fluentd-loggly" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
