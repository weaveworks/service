FROM grafana/grafana
LABEL works.weave.role=system

COPY grafana-defaults.ini grafana.ini /etc/grafana/
COPY saas-components.json /grafana/dashboards/

EXPOSE 3000
# ENTRYPOINT is the default
