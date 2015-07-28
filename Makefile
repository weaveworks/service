.PHONY: all build test integration-test clean

# If you can use Docker without being root, you can `make SUDO= <target>`
WEB_UPTODATE=.web.uptodate
WEB_IMAGE=weaveworks/web

IMAGES=$(WEB_IMAGE)
UPTODATES=$(WEB_UPTODATE)

SUDO=sudo
ENV=development

all: build

build: $(UPTODATES)

$(WEB_UPTODATE): web/Dockerfile web/templates/*
	$(SUDO) docker-compose build web
	touch $@

test:
	go test ./...

integration-test: $(WEB_UPTODATE)
	$(SUDO) docker-compose run --rm web go test -tags integration -timeout 30s ./...

clean:
	-$(SUDO) docker rmi $(IMAGES)
	rm -rf $(EXES) $(UPTODATES)
	go clean ./...
