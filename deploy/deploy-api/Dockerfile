FROM alpine
MAINTAINER Weaveworks Inc <help@weave.works>
WORKDIR /
COPY deploy-api /
COPY migrations /migrations/
EXPOSE 80
ENTRYPOINT ["/deploy-api"]
