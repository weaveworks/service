.PHONY: all test integration-test clean

# If you can use Docker without being root, you can `make SUDO= <target>`
IMAGE_TAR=image.tar
APP_MAPPER_IMAGE=weaveworks/app-mapper
APP_MAPPER_DB_IMAGE=weaveworks/app-mapper-db
APP_MAPPER_EXE=app-mapper
SUDO=sudo

all: $(IMAGE_TAR)

$(IMAGE_TAR): Dockerfile $(APP_MAPPER_EXE) db/Dockerfile db/schema.sql
	$(SUDO) docker build -t $(APP_MAPPER_IMAGE) .
	$(SUDO) docker build -t $(APP_MAPPER_DB_IMAGE) db
	$(SUDO) docker save $(APP_MAPPER_IMAGE):latest $(APP_MAPPER_DB_IMAGE):latest > $@

$(APP_MAPPER_EXE): *.go
	go get -d ./...
	go build -ldflags "-extldflags \"-static\" -linkmode=external" -o $@ ./$(@D)

integration-test:
	script/integration-test.sh
test:
	go test ./...

clean:
	-$(SUDO) docker rmi $(APP_MAPPER_IMAGE) $(APP_MAPPER_DB_IMAGE)
	rm -rf $(APP_MAPPER_EXE) $(IMAGE_TAR)
	go clean ./...
