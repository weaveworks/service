FROM mhart/alpine-node:8.4
MAINTAINER Weaveworks Inc <help@weave.works>
RUN apk add --no-cache git openssh
RUN git config --global user.email "team+gitbot@weave.works"
ENV GIT_SSH_COMMAND "ssh -oStrictHostKeyChecking=no"
WORKDIR /
COPY service-ui-kicker /
EXPOSE 80
ENTRYPOINT ["/service-ui-kicker"]
