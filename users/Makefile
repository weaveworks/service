.PHONY: all test integration-test clean

IMAGE_TAR=image.tar
WEB_IMAGE=weaveworks/users
USERS_EXE=users

ENV=development

all: $(IMAGE_TAR)

$(IMAGE_TAR): Dockerfile *.go templates/* $(USERS_EXE)
	docker build -t $(WEB_IMAGE) .
	docker save $(WEB_IMAGE):latest > $@

$(USERS_EXE): *.go
	go get -tags netgo ./$(@D)
	go build -ldflags "-extldflags \"-static\"" -tags netgo -o $@ ./$(@D)

test:
	go test ./...

integration-test: $(WEB_UPTODATE)
	docker-compose up -d
	docker-compose run --rm users go test -tags integration -timeout 30s ./...

clean:
	-docker rmi $(WEB_IMAGE)
	rm -rf $(EXES) $(IMAGE_TAR)
	go clean ./...
