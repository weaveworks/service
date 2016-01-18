FROM debian:jessie
LABEL works.weave.role=system
RUN apt-get -y update && apt-get -y install curl
COPY loadgen /
ENTRYPOINT ["/loadgen"]
