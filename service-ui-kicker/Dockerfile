FROM node:8.4.0-alpine
MAINTAINER Weaveworks Inc <help@weave.works>
RUN apk add --no-cache python make gcc g++ git openssh
RUN git config --global user.email "team+gitbot@weave.works"
ENV GIT_SSH_COMMAND "ssh -oStrictHostKeyChecking=no"
WORKDIR /
COPY service-ui-kicker /
EXPOSE 80
ENTRYPOINT ["/service-ui-kicker"]
