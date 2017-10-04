FROM alpine:3.4
EXPOSE 80
ENTRYPOINT ["/uploader"]
RUN apk add --update \
    ca-certificates \
  && rm -rf /var/cache/apk/*
ADD migrations /migrations
ADD uploader /
