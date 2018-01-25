FROM       alpine:3.4
MAINTAINER Weaveworks Inc <help@weave.works>
WORKDIR    /
COPY       cmd/configmanager/configmanager /
COPY       migrations /migrations
EXPOSE     80
ENTRYPOINT [ "/configmanager" ]
