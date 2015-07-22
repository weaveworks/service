.PHONY: all deps build test clean

# If you can use Docker without being root, you can `make SUDO= <target>`
SUDO=sudo
APP_EXE=app
APP_UPTODATE=.app.uptodate
APP_IMAGE=weaveworks/weave-run

all: deps build

deps:
	go get -v -t ./...

build: $(APP_UPTODATE)

$(APP_UPTODATE): Dockerfile $(APP_EXE) templates/*
	$(SUDO) docker build -t $(APP_IMAGE) .
	touch $@

$(APP_EXE): *.go
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $@ ./$(@D)

test:
	go test ./...

clean:
	-$(SUDO) docker rmi $(APP_IMAGE)
	rm -rf $(APP_EXE) $(APP_UPTODATE)
	go clean ./...
