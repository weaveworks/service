FROM fluent/fluentd:v0.14.20-debian
USER root
RUN buildDeps="sudo make gcc g++ libc-dev ruby-dev autoconf automake libtool libltdl-dev" \
 && apt-get update \
 && apt-get install -y --no-install-recommends $buildDeps \
 && sudo gem install \
         json:2.1.0 \
         fluent-plugin-loggly:0.0.9 \
         fluent-plugin-kubernetes_metadata_filter:1.0.1 \
         fluent-plugin-cloudwatch-logs:0.4.4 \
         fluent-plugin-systemd:0.3.1 \
         fluent-plugin-concat:2.2.0 \
         fluent-plugin-fields-parser:0.1.2 \
         fluent-plugin-rewrite-tag-filter:2.0.2 \
         fluent-plugin-elasticsearch:2.8.2 \
 && sudo gem sources --clear-all \
 && SUDO_FORCE_REMOVE=yes \
    apt-get purge -y --auto-remove \
                  -o APT::AutoRemove::RecommendsImportant=false \
                  $buildDeps \
 && rm -rf /var/lib/apt/lists/* \
           /home/fluent/.gem/ruby/2.3.0/cache/*.gem

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="fluentd-loggly" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/fluentd-loggly" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
