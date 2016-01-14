FROM alpine:latest
LABEL works.weave.role=system
RUN apk update && apk upgrade && apk add curl bash
COPY loadgen /
ENTRYPOINT ["/loadgen"]
