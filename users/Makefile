.PHONY: all build test integration-test clean

# If you can use Docker without being root, you can `make SUDO= <target>`
WEB_UPTODATE=.users.uptodate
WEB_IMAGE=weaveworks/users

IMAGES=$(WEB_IMAGE)
UPTODATES=$(WEB_UPTODATE)

SUDO=sudo
ENV=development

all: build

build: $(UPTODATES)

$(WEB_UPTODATE): Dockerfile *.go templates/*
	$(SUDO) docker-compose build users
	touch $@

test:
	go test ./...

integration-test: $(WEB_UPTODATE)
	$(SUDO) docker-compose run --rm users go test -tags integration -timeout 30s ./...

clean:
	-$(SUDO) docker rmi $(IMAGES)
	rm -rf $(EXES) $(UPTODATES)
	go clean ./...
