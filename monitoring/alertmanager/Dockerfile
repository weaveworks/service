FROM debian:jessie
LABEL works.weave.role=system

COPY run /alertmanager/
COPY alertmanager.yml /etc/alertmanager/
ADD https://github.com/prometheus/alertmanager/releases/download/0.1.0-beta0/alertmanager-0.1.0-beta0.linux-amd64.tar.gz /alertmanager/alertmanager.tar.gz
RUN tar zxf /alertmanager/alertmanager.tar.gz -C /alertmanager

EXPOSE 9093
ENTRYPOINT ["/alertmanager/run"]
