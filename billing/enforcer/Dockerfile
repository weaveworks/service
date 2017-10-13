FROM alpine:3.6
EXPOSE 80
ENTRYPOINT ["/enforcer"]
RUN apk add --update \
    ca-certificates \
  && rm -rf /var/cache/apk/*
ADD enforcer /
