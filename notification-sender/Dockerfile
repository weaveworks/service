FROM alpine:3.6
MAINTAINER Weaveworks Inc <help@weave.works>
RUN apk add --no-cache ca-certificates
WORKDIR /
COPY cmd/sender/sender /
EXPOSE 80
ENTRYPOINT ["/sender"]
