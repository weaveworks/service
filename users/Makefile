.PHONY: all build test integration-test clean

WEB_UPTODATE=.users.uptodate
WEB_IMAGE=weaveworks/users

IMAGES=$(WEB_IMAGE)
UPTODATES=$(WEB_UPTODATE)

ENV=development

all: build

build: $(UPTODATES)

$(WEB_UPTODATE): Dockerfile *.go templates/*
	docker-compose build users
	touch $@

test:
	go test ./...

integration-test: $(WEB_UPTODATE)
	docker-compose run --rm users go test -tags integration -timeout 30s ./...

clean:
	-docker rmi $(IMAGES)
	rm -rf $(EXES) $(UPTODATES)
	go clean ./...
