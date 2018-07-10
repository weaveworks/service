FROM httpd:2.4-alpine
COPY . /usr/local/apache2/htdocs/

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="billing-hpm_demo" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/billing-hpm_demo" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
