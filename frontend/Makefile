IMAGE_TAR=image.tar
IMAGE_NAME=weaveworks/frontend

$(IMAGE_TAR): Dockerfile default.conf
	docker build -t $(IMAGE_NAME) .
	docker save $(IMAGE_NAME):latest > $@

clean:
	-docker rmi $(IMAGE_NAME) > /dev/null 2>&1 || true
	-rm -f $(IMAGE_TAR)
