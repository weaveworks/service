FROM node:8.4.0-alpine
RUN apk add --no-cache python make gcc g++ git openssh
RUN git config --global user.email "team+gitbot@weave.works"
ENV GIT_SSH_COMMAND "ssh -oStrictHostKeyChecking=no"
WORKDIR /
COPY service-ui-kicker /
EXPOSE 80
ENTRYPOINT ["/service-ui-kicker"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="service-ui-kicker" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/service-ui-kicker" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
