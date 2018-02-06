FROM alpine:3.6
MAINTAINER Weaveworks Inc <help@weave.works>
RUN apk add --no-cache ca-certificates
WORKDIR /
COPY cmd/eventmanager/eventmanager /
EXPOSE 80
ENTRYPOINT ["/eventmanager"]
