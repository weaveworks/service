FROM prom/prometheus
ADD prometheus /bin/
ADD prometheus.yml /etc/prometheus/
EXPOSE 80
CMD [ "-config.file=/etc/prometheus/prometheus.yml", \
      "-storage.local.path=/prometheus", \
      "-web.console.libraries=/etc/prometheus/console_libraries", \
      "-web.console.templates=/etc/prometheus/consoles", \
      "-web.listen-address=:80" ]
