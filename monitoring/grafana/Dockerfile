FROM grafana/grafana
LABEL works.weave.role=system

COPY *.ini /etc/grafana/
COPY *.json /grafana/dashboards/

EXPOSE 80
ENTRYPOINT ["/usr/sbin/grafana-server", "--homepath=/usr/share/grafana", "--config=/etc/grafana/grafana.ini"]
