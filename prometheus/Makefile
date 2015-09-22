.PHONY: all clean

IMAGE_TAR=image.tar
IMAGE_NAME=weaveworks/prometheus
PROMETHEUS_DIR=.prometheus
PRINT_HEAD_SHA=$$(cd $(PROMETHEUS_DIR) && git rev-parse --short HEAD)
HEAD_IMAGE_UPTODATE=.head.image.updodate

all: $(IMAGE_TAR)

$(IMAGE_TAR): Dockerfile $(HEAD_IMAGE_UPTODATE)
	docker build -t $(IMAGE_NAME) .
	docker save $(IMAGE_NAME):latest > $@

$(HEAD_IMAGE_UPTODATE): $(PROMETHEUS_DIR)
	make -C $(PROMETHEUS_DIR) docker
	docker tag prometheus:${PRINT_HEAD_SHA} prometheus:head
	touch $@

$(PROMETHEUS_DIR):
	git clone https://github.com/prometheus/prometheus $(PROMETHEUS_DIR)

clean:
	-docker rmi $(IMAGE_NAME) prometheus:head prometheus:${PRINT_HEAD_SHA}
	rm -rf $(IMAGE_TAR) $(PROMETHEUS_EXE) $(PROMETHEUS_DIR) $(HEAD_IMAGE_UPTODATE)
