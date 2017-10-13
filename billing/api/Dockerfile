FROM alpine:3.6
EXPOSE 80
ENTRYPOINT ["/api"]
RUN apk add --update \
    ca-certificates \
  && rm -rf /var/cache/apk/*
ADD migrations /migrations
ADD api /
