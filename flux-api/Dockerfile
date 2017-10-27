FROM alpine:3.6
WORKDIR /home/flux
ENTRYPOINT [ "/sbin/tini", "--", "flux-api" ]
RUN apk add --no-cache ca-certificates tini
ADD migrations.tar /home/flux
COPY flux-api /usr/local/bin/
