FROM golang:1.5.1
RUN go clean -i net && go install -tags netgo std
RUN apt-get update && apt-get install -y python-pip && pip install requests
ENV GO15VENDOREXPERIMENT 1
COPY build.sh /
ENTRYPOINT ["/build.sh"]
