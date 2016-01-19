IMAGES_UPTODATE=.images.uptodate
IMAGE_PREFIX=quay.io/weaveworks

.PHONY: all clean

all: ${IMAGES_UPTODATE}

${IMAGES_UPTODATE}: prometheus/${IMAGES_UPTODATE} grafana/${IMAGES_UPTODATE} gfdatasource/${IMAGES_UPTODATE} loadgen/${IMAGES_UPTODATE} alertmanager/${IMAGES_UPTODATE}
	touch $@

prometheus/${IMAGES_UPTODATE}: prometheus/Dockerfile prometheus/alert.rules prometheus/prometheus.yml prometheus/run
	docker build -t ${IMAGE_PREFIX}/prometheus prometheus
	touch $@

grafana/${IMAGES_UPTODATE}: grafana/Dockerfile grafana/grafana-defaults.ini grafana/grafana.ini grafana/saas-components.json
	docker build -t ${IMAGE_PREFIX}/grafana grafana
	touch $@

gfdatasource/${IMAGES_UPTODATE}: gfdatasource/Dockerfile gfdatasource/gfdatasource
	docker build -t ${IMAGE_PREFIX}/gfdatasource gfdatasource
	touch $@

loadgen/${IMAGES_UPTODATE}: loadgen/Dockerfile loadgen/loadgen
	docker build -t ${IMAGE_PREFIX}/loadgen loadgen
	touch $@

alertmanager/${IMAGES_UPTODATE}: alertmanager/Dockerfile alertmanager/alertmanager-local.yml alertmanager/alertmanager-dev.yml alertmanager/alertmanager-prod.yml
	docker build -t ${IMAGE_PREFIX}/alertmanager alertmanager
	touch $@

clean:
	-docker rmi ${IMAGE_PREFIX}/prometheus   >/dev/null 2>&1 || true
	-docker rmi ${IMAGE_PREFIX}/grafana      >/dev/null 2>&1 || true
	-docker rmi ${IMAGE_PREFIX}/gfdatasource >/dev/null 2>&1 || true
	-docker rmi ${IMAGE_PREFIX}/loadgen      >/dev/null 2>&1 || true
	-docker rmi ${IMAGE_PREFIX}/alertmanager >/dev/null 2>&1 || true
	rm -f ${IMAGES_UPTODATE} */${IMAGES_UPTODATE}
