FROM debian:jessie
LABEL works.weave.role=system
RUN apt-get update -y && apt-get install -y ca-certificates curl
RUN curl -L https://github.com/prometheus/alertmanager/releases/download/0.2.1/alertmanager-0.2.1.linux-amd64.tar.gz | tar xz -C / && mv /alertmanager-0.2.1.linux-amd64 /alertmanager
COPY alertmanager-*.yml /etc/alertmanager/
EXPOSE 80
ENTRYPOINT ["/alertmanager/alertmanager"]
