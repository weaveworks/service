FROM fluent/fluentd:v0.14.2

USER root
RUN apk --no-cache --update add build-base ruby-dev && \
    gem install json fluent-plugin-loggly fluent-plugin-kubernetes_metadata_filter && \
    apk del build-base ruby-dev && \
    rm -rf /tmp/* /var/tmp/* /var/cache/apk/* /usr/lib/ruby/gems/*/cache/*.gem
