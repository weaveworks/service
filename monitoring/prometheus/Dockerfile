FROM prom/prometheus
LABEL works.weave.role=system

COPY prometheus.yml alert.rules /etc/prometheus/
COPY run /prometheus/

EXPOSE 9090
ENTRYPOINT ["/prometheus/run"]
