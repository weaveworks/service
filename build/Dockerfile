FROM golang:1.6.0
RUN go clean -i net && go install -tags netgo std
RUN apt-get update && apt-get install -y python-pip && pip install requests
COPY build.sh /
ENTRYPOINT ["/build.sh"]
