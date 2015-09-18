.PHONY: all test integration-test clean

IMAGE_TAR=image.tar
IMAGE_NAME=weaveworks/users
USERS_EXE=users
ENV=development
DB_IMAGE_TAR=db_image.tar
DB_IMAGE_NAME=weaveworks/db_image

all: $(IMAGE_TAR)

$(IMAGE_TAR): Dockerfile templates/* $(USERS_EXE) db/* ca-certificates.crt
	docker build -t $(IMAGE_NAME) .
	docker build -t $(DB_IMAGE_NAME) db/
	docker save $(IMAGE_NAME):latest $(DB_IMAGE_NAME):latest  > $@

ca-certificates.crt: /etc/ssl/certs/ca-certificates.crt
	cp $? $@

$(USERS_EXE): *.go
	go get -d ./...
	go build -ldflags "-extldflags \"-static\" -linkmode=external" -o $@ ./$(@D)

test:
	go test ./...

integration-test: $(WEB_UPTODATE)
	docker-compose up -d
	docker run --rm \
		-v "$(shell go list -e -f {{.Dir}} github.com/weaveworks/service)":/usr/src/go/src/github.com/weaveworks/service \
		--workdir /usr/src/go/src/github.com/weaveworks/service/users \
		golang:1.4 \
		/bin/bash -c "go get -v -t ./... ; go test -tags integration -timeout 30s ./..."

clean:
	-docker rmi $(IMAGE_NAME)
	rm -rf $(USERS_EXE) $(IMAGE_TAR)
	go clean ./...
