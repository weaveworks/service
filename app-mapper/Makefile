.PHONY: all deps build test clean

# If you can use Docker without being root, you can `make SUDO= <target>`
APP_MAPPER_UPTODATE=.app-mapper.uptodate
APP_MAPPER_IMAGE=weaveworks/app-mapper
APP_MAPPER_EXE=app-mapper

SUDO=sudo

all: deps build

deps:
	go get -v -t ./$*/...

build: $(APP_MAPPER_UPTODATE)

$(APP_MAPPER_UPTODATE): Dockerfile $(APP_MAPPER_EXE)
	$(SUDO) docker build -t $(APP_MAPPER_IMAGE) .
	touch $@

$(APP_MAPPER_EXE): *.go
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $@ ./$(@D)

test:
	go test ./...

clean:
	-$(SUDO) docker rmi $(APP_MAPPER_IMAGE)
	rm -rf $(APP_MAPPER_EXE) $(APP_MAPPER_UPTODATE)
	go clean ./...
