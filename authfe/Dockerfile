FROM alpine:3.6
MAINTAINER Weaveworks Inc <help@weave.works>
WORKDIR /
COPY authfe /
COPY ca-certificates.crt /etc/ssl/certs/
EXPOSE 80
ENTRYPOINT ["/authfe"]
