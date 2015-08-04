.PHONY: all test clean

# If you can use Docker without being root, you can `make SUDO= <target>`
IMAGE_TAR=image.tar
APP_MAPPER_IMAGE=weaveworks/app-mapper
APP_MAPPER_EXE=app-mapper

SUDO=sudo

all: $(IMAGE_TAR)

$(IMAGE_TAR): Dockerfile $(APP_MAPPER_EXE)
	$(SUDO) docker build -t $(APP_MAPPER_IMAGE) .
	$(SUDO) docker save $(APP_MAPPER_IMAGE):latest > $@

$(APP_MAPPER_EXE): *.go
	go get -tags netgo ./$(@D)
	go build -ldflags "-extldflags \"-static\"" -tags netgo -o $@ ./$(@D)

test:
	go test ./...

clean:
	-$(SUDO) docker rmi $(APP_MAPPER_IMAGE)
	rm -rf $(APP_MAPPER_EXE) $(IMAGE_TAR)
	go clean ./...
