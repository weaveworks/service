FROM alpine:3.4
MAINTAINER Weaveworks Inc <help@weave.works>
WORKDIR /
COPY cmd/users/users /
COPY templates/*.html /templates/
COPY templates/*.text /templates/
COPY db/migrations /migrations/
COPY ca-certificates.crt /etc/ssl/certs/
EXPOSE 80
ENTRYPOINT ["/users"]
