FROM alpine:3.4
MAINTAINER Weaveworks Inc <help@weave.works>
WORKDIR /
COPY cmd/prom/prom /
COPY db/migrations /migrations/
EXPOSE 80
ENTRYPOINT ["/prom"]
