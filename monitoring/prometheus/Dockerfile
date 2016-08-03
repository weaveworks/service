FROM prom/prometheus:v1.0.0
LABEL works.weave.role=system

COPY prometheus.yml *.rules /etc/prometheus/
COPY run /prometheus/

EXPOSE 80
ENTRYPOINT ["/prometheus/run"]
