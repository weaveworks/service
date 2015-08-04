.PHONY: all test integration-test clean

IMAGE_TAR=image.tar
IMAGE_NAME=weaveworks/users
USERS_EXE=users

ENV=development

all: $(IMAGE_TAR)

$(IMAGE_TAR): Dockerfile *.go templates/* $(USERS_EXE)
	docker build -t $(IMAGE_NAME) .
	docker save $(IMAGE_NAME):latest > $@

$(USERS_EXE): *.go
	go get -tags netgo ./$(@D)
	go build -ldflags "-extldflags \"-static\"" -tags netgo -o $@ ./$(@D)

test:
	go test ./...

integration-test: $(WEB_UPTODATE)
	docker-compose up -d
	docker-compose run --rm users go test -tags integration -timeout 30s ./...

clean:
	-docker rmi $(IMAGE_NAME)
	rm -rf $(EXES) $(IMAGE_TAR)
	go clean ./...
