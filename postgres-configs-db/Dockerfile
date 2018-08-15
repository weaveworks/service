FROM postgres:9.6
ENV POSTGRES_DB configs
ADD /postgres-configs-db/init.sql /docker-entrypoint-initdb.d/

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="billing-api" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/postgres-configs-db" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
