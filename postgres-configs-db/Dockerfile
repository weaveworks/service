FROM postgres:10.6
ENV POSTGRES_DB configs
ADD init.sql /docker-entrypoint-initdb.d/

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="billing-api" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/main/postgres-configs-db" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
