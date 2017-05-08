FROM alpine:3.5
MAINTAINER Weaveworks Inc <help@weave.works>
WORKDIR /
COPY cmd/notebooks/notebooks /
COPY db/migrations /migrations/
EXPOSE 80
ENTRYPOINT ["/notebooks"]
