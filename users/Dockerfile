FROM scratch
MAINTAINER Weaveworks Inc <help@weave.works>
WORKDIR /
COPY users /
COPY templates /templates/
COPY db/migrations /migrations/
COPY ca-certificates.crt /etc/ssl/certs/
EXPOSE 80
ENTRYPOINT ["/users"]
