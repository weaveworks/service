IMAGES_UPTODATE=.images.uptodate
IMAGE_PREFIX=quay.io/weaveworks
IMAGE_TAG:=$(shell ../image-tag)

.PHONY: all clean

all: ${IMAGES_UPTODATE}

${IMAGES_UPTODATE}: prometheus/${IMAGES_UPTODATE} grafana/${IMAGES_UPTODATE} gfdatasource/${IMAGES_UPTODATE} loadgen/${IMAGES_UPTODATE} alertmanager/${IMAGES_UPTODATE}
	touch $@

prometheus/${IMAGES_UPTODATE}: prometheus/*
	docker build -t ${IMAGE_PREFIX}/prometheus prometheus
	docker tag "${IMAGE_PREFIX}/prometheus" "${IMAGE_PREFIX}/prometheus:${IMAGE_TAG}"
	touch $@

grafana/${IMAGES_UPTODATE}: grafana/*
	docker build -t ${IMAGE_PREFIX}/grafana grafana
	docker tag "${IMAGE_PREFIX}/grafana" "${IMAGE_PREFIX}/grafana:${IMAGE_TAG}"
	touch $@

gfdatasource/${IMAGES_UPTODATE}: gfdatasource/Dockerfile gfdatasource/gfdatasource
	docker build -t ${IMAGE_PREFIX}/gfdatasource gfdatasource
	docker tag "${IMAGE_PREFIX}/gfdatasource" "${IMAGE_PREFIX}/gfdatasource:${IMAGE_TAG}"
	touch $@

loadgen/${IMAGES_UPTODATE}: loadgen/Dockerfile loadgen/loadgen
	docker build -t ${IMAGE_PREFIX}/loadgen loadgen
	docker tag "${IMAGE_PREFIX}/loadgen" "${IMAGE_PREFIX}/loadgen:${IMAGE_TAG}"
	touch $@

alertmanager/${IMAGES_UPTODATE}: alertmanager/Dockerfile alertmanager/alertmanager-local.yml alertmanager/alertmanager-dev.yml alertmanager/alertmanager-prod.yml
	docker build -t ${IMAGE_PREFIX}/alertmanager alertmanager
	docker tag "${IMAGE_PREFIX}/alertmanager" "${IMAGE_PREFIX}/alertmanager:${IMAGE_TAG}"
	touch $@

clean:
	-docker rmi ${IMAGE_PREFIX}/prometheus   >/dev/null 2>&1 || true
	-docker rmi ${IMAGE_PREFIX}/grafana      >/dev/null 2>&1 || true
	-docker rmi ${IMAGE_PREFIX}/gfdatasource >/dev/null 2>&1 || true
	-docker rmi ${IMAGE_PREFIX}/loadgen      >/dev/null 2>&1 || true
	-docker rmi ${IMAGE_PREFIX}/alertmanager >/dev/null 2>&1 || true
	rm -f ${IMAGES_UPTODATE} */${IMAGES_UPTODATE}
