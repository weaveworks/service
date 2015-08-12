.PHONY: all clean

IMAGE_TAR=image.tar
IMAGE_NAME=weaveworks/prometheus
PROMETHEUS_EXE=prometheus
PROMETHEUS_DIR=.prometheus

all: $(IMAGE_TAR)

$(IMAGE_TAR): Dockerfile $(PROMETHEUS_EXE)
	docker build -t $(IMAGE_NAME) .
	docker save $(IMAGE_NAME):latest > $@

$(PROMETHEUS_EXE): $(PROMETHEUS_DIR)
	make -C $(PROMETHEUS_DIR)
	cp $(PROMETHEUS_DIR)/$(PROMETHEUS_EXE) .

$(PROMETHEUS_DIR):
	git clone https://github.com/prometheus/prometheus $(PROMETHEUS_DIR)

clean:
	-docker rmi $(IMAGE_NAME)
	rm -rf $(IMAGE_TAR) $(PROMETHEUS_EXE) $(PROMETHEUS_DIR)
