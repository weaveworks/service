IMAGES_UPTODATE=.images.uptodate
IMAGE_NAME=quay.io/weaveworks/monitoring
PROMETHEUS_VERSION=0.16.1
ALERTMANAGER_VERSION=0.1.0-beta0
GRAFANA_VERSION=2.5.0

.PHONY: container clean

container: $(IMAGES_UPTODATE)

$(IMAGES_UPTODATE): Dockerfile alertmanager run-alertmanager prometheus run-prometheus prometheus.yml alert.rules grafana run-grafana grafana-defaults.ini grafana.ini saas-components.json public_gen/robots.txt gfdatasource run-gfdatasource loadgen run-loadgen runsvinit
	docker build -t ${IMAGE_NAME} .
	touch $@

#
# Prometheus, 0.16.0+
#

prometheus: prometheus-${PROMETHEUS_VERSION}.linux-amd64/prometheus
	cp $< $@

prometheus-${PROMETHEUS_VERSION}.linux-amd64/prometheus: prometheus-${PROMETHEUS_VERSION}.linux-amd64.tar.gz
	tar zxf $<
	touch $@

prometheus-${PROMETHEUS_VERSION}.linux-amd64.tar.gz:
	wget --quiet https://github.com/prometheus/prometheus/releases/download/${PROMETHEUS_VERSION}/prometheus-${PROMETHEUS_VERSION}.linux-amd64.tar.gz
	touch $@

#
# Alertmanager, 0.1+
#

alertmanager: alertmanager-${ALERTMANAGER_VERSION}.linux-amd64.tar.gz
	tar zxf $<
	touch $@

alertmanager-${ALERTMANAGER_VERSION}.linux-amd64.tar.gz:
	wget --quiet https://github.com/prometheus/alertmanager/releases/download/${ALERTMANAGER_VERSION}/alertmanager-${ALERTMANAGER_VERSION}.linux-amd64.tar.gz
	touch $@

#
# Grafana, 2.5.0+
#

grafana: grafana-${GRAFANA_VERSION}/bin/grafana-server
	cp $< $@

# robots.txt is a shibboleth for the entire public directory
public_gen/robots.txt: grafana-${GRAFANA_VERSION}/public/robots.txt
	cp -r $(shell dirname $<) $(shell dirname $@)

grafana-defaults.ini: grafana-${GRAFANA_VERSION}/conf/defaults.ini
	cp $< $@

grafana-${GRAFANA_VERSION}/bin/grafana-server: grafana-${GRAFANA_VERSION}/README.md

grafana-${GRAFANA_VERSION}/conf/defaults.ini: grafana-${GRAFANA_VERSION}/README.md

grafana-${GRAFANA_VERSION}/public/robots.txt: grafana-${GRAFANA_VERSION}/README.md

# README.md is a standin for the entire Grafana directory
grafana-${GRAFANA_VERSION}/README.md: grafana-${GRAFANA_VERSION}.linux-x64.tar.gz
	tar zxf $<
	touch $@


grafana-${GRAFANA_VERSION}.linux-x64.tar.gz:
	wget --quiet https://grafanarel.s3.amazonaws.com/builds/grafana-${GRAFANA_VERSION}.linux-x64.tar.gz
	touch $@

#
# runsvinit
#

runsvinit:
	go get -u -v github.com/peterbourgon/runsvinit # fetch repo
	env GOOS=linux GOARCH=amd64 go build github.com/peterbourgon/runsvinit # build in pwd

clean:
	-docker rmi $(IMAGE_NAME) >/dev/null 2>&1 || true
	rm -f $(IMAGES_UPTODATE)
	rm -rf prometheus prometheus-*.linux-amd64* || true
	rm -rf alertmanager alertmanager-*.linux-amd64* || true
	rm -rf grafana grafana-*.*.* || true
