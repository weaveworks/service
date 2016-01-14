FROM grafana/grafana
LABEL works.weave.role=system

COPY grafana-defaults.ini grafana.ini /grafana/conf/
COPY saas-components.json /grafana/dashboards/

EXPOSE 3000
ENTRYPOINT ["/grafana/grafana"]
