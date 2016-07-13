FROM alpine:3.4
LABEL works.weave.role=system
RUN apk update && apk upgrade && apk add curl bash
COPY gfdatasource /
ENTRYPOINT ["/gfdatasource"]
