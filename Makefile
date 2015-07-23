.PHONY: all deps build test clean

# If you can use Docker without being root, you can `make SUDO= <target>`
SUDO=sudo
WEB_EXE=web/web
WEB_UPTODATE=.web.uptodate
WEB_IMAGE=weaveworks/web
DEV_IMAGE=weaveworks/dev
DEV_UPTODATE=.dev.uptodate

IMAGES=$(WEB_IMAGE) $(DEV_IMAGE)
UPTODATES=$(WEB_UPTODATE) $(DEV_UPTODATE)

all: deps build

deps:
	go get -v -t ./web/...
	go get -v bitbucket.org/liamstask/goose/cmd/goose

build: $(WEB_UPTODATE) $(DEV_UPTODATE)

$(WEB_UPTODATE): web/Dockerfile $(WEB_EXE) web/templates/*
	$(SUDO) docker build -t $(WEB_IMAGE) web
	touch $@

$(DEV_UPTODATE): $(WEB_UPTODATE) Dockerfile db/dbconf.yml db/migrations/*
	$(SUDO) docker build -t $(DEV_IMAGE) .
	touch $@

$(WEB_EXE): web/*.go
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $@ ./$(@D)

test:
	go test ./web/...

integration-test: $(DEV_UPTODATE)
	$(SUDO) ./bin/dev make remote-integration-test

remote-integration-test: deps
	go test -tags integration -timeout 30s ./web/...

clean:
	-$(SUDO) docker rmi $(WEB_IMAGE) $(IMAGES)
	rm -rf $(WEB_EXE) $(UPTODATES)
	go clean ./web/...
