.PHONY: all deps build test clean

# If you can use Docker without being root, you can `make SUDO= <target>`
SUDO=sudo
WEB_EXE=web/web
WEB_UPTODATE=.web.uptodate
WEB_IMAGE=weaveworks/web

all: deps build

deps:
	go get -v -t ./web/...

build: $(WEB_UPTODATE)

$(WEB_UPTODATE): web/Dockerfile $(WEB_EXE) web/templates/*
	$(SUDO) docker build -t $(WEB_IMAGE) web
	touch $@

$(WEB_EXE): web/*.go
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $@ ./$(@D)

test:
	go test ./web/...

clean:
	-$(SUDO) docker rmi $(WEB_IMAGE)
	rm -rf $(WEB_EXE) $(WEB_UPTODATE)
	go clean ./web/...
