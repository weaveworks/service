.PHONY: all clean

IMAGE_UPTODATE=.image.uptodate
IMAGE_NAME=quay.io/weaveworks/frontend

all: $(IMAGE_UPTODATE)

$(IMAGE_UPTODATE): Dockerfile default.conf
	docker build -t $(IMAGE_NAME) .
	touch $@

clean:
	-docker rmi $(IMAGE_NAME) > /dev/null 2>&1 || true
	-rm -f $(IMAGE_UPTODATE)

