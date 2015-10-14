IMAGES_UPTODATE=.images.uptodate
IMAGE_NAME=quay.io/weaveworks/monitoring
PROMETHEUS_VERSION=0.16.0rc2
GRAFANA_REV=0db2d07a1a
GRAFANA_GO=GOPATH=$(shell pwd)/.grafana_gopath go
GRAFANA_GODEP=GOPATH=$(shell pwd)/.grafana_gopath $(shell pwd)/.grafana_gopath/bin/godep

.PHONY: all clean

all: $(IMAGES_UPTODATE)

$(IMAGES_UPTODATE): Dockerfile prometheus run-prometheus prometheus.yml grafana run-grafana grafana-defaults.ini grafana.ini saas-components.json public_gen/robots.txt gfdatasource run-gfdatasource loadgen run-loadgen runsvinit
	docker build -t ${IMAGE_NAME} .
	touch $@

#
# Prometheus. We just need to download version 0.16.0 or above.
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
# Grafana. We need to build this from source, from the 'prometheus' branch, to
# get support for Prometheus as a data source. We use a separate GOPATH,
# because the Grafana build instructions do a `godep restore` into it, which
# can pollute it.
#

grafana: .grafana_gopath/bin/grafana
	cp $< $@

grafana-defaults.ini: .grafana_gopath/src/github.com/grafana/grafana/conf/defaults.ini
	cp -r $< $@

# robots.txt is a stand-in for public_gen, to avoid using the directory as a dependency.
public_gen/robots.txt: .grafana_gopath/src/github.com/grafana/grafana/public_gen/robots.txt
	cp -r $(shell dirname $<) $(shell dirname $@)

.grafana_gopath/src/github.com/grafana/grafana/conf/defaults.ini: .grafana_gopath/src/github.com/grafana/.uptodate

.grafana_gopath/src/github.com/grafana/grafana/public_gen/robots.txt: .grafana_gopath/src/github.com/grafana/.uptodate
	docker run -ti -v $(shell pwd)/:/home/weave node:0.10 /bin/sh -c "cd /home/weave/.grafana_gopath/src/github.com/grafana/grafana && npm install && npm install -g grunt-cli && grunt"

.grafana_gopath/bin/grafana: .grafana_gopath/src/github.com/grafana/.uptodate
	${GRAFANA_GO} install github.com/grafana/grafana

.grafana_gopath/src/github.com/grafana/.uptodate: .grafana_gopath/src/github.com/grafana/grafana/build.go .grafana_gopath/bin/godep
	@uname -a | grep Linux || { \
		echo ""; \
		echo "Grafana can only be built on Linux. Very sorry."; \
		echo ""; \
		false; \
	}
	cd $(shell dirname $@)/grafana && git checkout ${GRAFANA_REV} && ${GRAFANA_GO} run build.go setup && ${GRAFANA_GODEP} restore
	touch $@

.grafana_gopath/bin/godep:
	${GRAFANA_GO} get github.com/kr/godep

.grafana_gopath/src/github.com/grafana/grafana/build.go:
	${GRAFANA_GO} get -d github.com/grafana/grafana

#
# runsvinit
#

runsvinit: $(GOPATH)/bin/runsvinit
	cp $< $@

$(GOPATH)/bin/runsvinit:
	GOOS=linux GOARCH=amd64 go get -u -v github.com/peterbourgon/runsvinit

clean:
	-docker rmi $(IMAGE_NAME) >/dev/null 2>&1 || true
	rm -f $(IMAGES_UPTODATE)
	rm -rf prometheus prometheus-*.linux-amd64* || true
	rm -rf grafana .grafana_gopath conf/ public_gen/ data/ || true
