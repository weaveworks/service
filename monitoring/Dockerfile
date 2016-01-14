# When I use alpine, there are some exec errors, and the binaries fail to
# start. It's probably solvable, with some effort.
#FROM alpine:latest
#RUN echo "http://dl-4.alpinelinux.org/alpine/edge/testing" >>/etc/apk/repositories && apk add --update runit && rm -rf /var/cache/apk/*

FROM debian:jessie
RUN touch /etc/inittab
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y -q runit curl

LABEL works.weave.role=system

COPY prometheus /prometheus/
COPY prometheus.yml /etc/prometheus/
COPY alert.rules /etc/prometheus/
COPY run-prometheus /etc/service/prometheus/run
EXPOSE 9090

COPY alertmanager /alertmanager/
COPY alertmanager.yml /etc/alertmanager/alertmanager.yml
COPY run-alertmanager /etc/service/alertmanager/run
EXPOSE 9093

COPY grafana /grafana/
COPY grafana-defaults.ini /grafana/conf/defaults.ini
COPY public_gen /grafana/public_gen
COPY grafana.ini /grafana/conf/custom.ini
COPY saas-components.json /grafana/dashboards/saas-components.json
COPY run-grafana /etc/service/grafana/run
EXPOSE 3000

COPY gfdatasource /grafana/
COPY run-gfdatasource /etc/service/gfdatasource/run

COPY loadgen /loadgen/
COPY run-loadgen /etc/service/loadgen/run

COPY runsvinit /
ENTRYPOINT ["/runsvinit"]

