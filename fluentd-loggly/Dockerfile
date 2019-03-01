FROM fluent/fluentd:v1.2.6-debian
USER root
RUN buildDeps="sudo make gcc g++ libc-dev ruby-dev autoconf automake libtool libltdl-dev" \
 && apt-get update \
 && apt-get install -y --no-install-recommends $buildDeps \
 && sudo gem install \
         json:2.1.0 \
         fluent-plugin-loggly:0.0.9 \
         fluent-plugin-kubernetes_metadata_filter:1.0.1 \
         fluent-plugin-cloudwatch-logs:0.4.4 \
         fluent-plugin-systemd:1.0.1 \
         fluent-plugin-concat:2.2.0 \
         fluent-plugin-fields-parser:0.1.2 \
         fluent-plugin-rewrite-tag-filter:2.0.2 \
         fluent-plugin-elasticsearch:2.11.11 \
 && sudo gem sources --clear-all \
 && SUDO_FORCE_REMOVE=yes \
    apt-get purge -y --auto-remove \
                  -o APT::AutoRemove::RecommendsImportant=false \
                  $buildDeps \
 && rm -rf /var/lib/apt/lists/* \
           /home/fluent/.gem/ruby/2.3.0/cache/*.gem

# HACK! Overwrite some bug-fixes, pending merges upstream
ADD https://raw.githubusercontent.com/weaveworks/logfmt-ruby/infinity-banned/lib/logfmt/parser.rb /var/lib/gems/2.3.0/gems/logfmt-0.0.8/lib/logfmt/parser.rb
ADD https://raw.githubusercontent.com/weaveworks/fluent-plugin-loggly/error-handling/lib/fluent/plugin/out_loggly_buffered.rb /var/lib/gems/2.3.0/gems/fluent-plugin-loggly-0.0.9/lib/fluent/plugin/out_loggly_buffered.rb

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="fluentd-loggly" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/fluentd-loggly" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
