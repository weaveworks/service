.PHONY: all test integration-test clean

IMAGE_TAR=image.tar
IMAGE_NAME=weaveworks/users
USERS_EXE=users
ENV=development
NETGO_CHECK=@strings $@ | grep cgo_stub\\\.go >/dev/null || { \
	rm $@; \
	echo "\nYour go standard library was built without the 'netgo' build tag."; \
	echo "To fix that, run"; \
	echo "    sudo go clean -i net"; \
	echo "    sudo go install -tags netgo std"; \
	false; \
}
DB_IMAGE_TAR=db_image.tar
DB_IMAGE_NAME=weaveworks/db_image

all: $(IMAGE_TAR)

$(IMAGE_TAR): Dockerfile templates/* $(USERS_EXE) db/*
	docker build -t $(IMAGE_NAME) .
	docker build -t $(DB_IMAGE_NAME) db/
	docker save $(IMAGE_NAME):latest $(DB_IMAGE_NAME):latest  > $@

$(USERS_EXE): *.go
	go get -tags netgo ./$(@D)
	go build -ldflags "-extldflags \"-static\" -linkmode=external" -tags netgo -o $@ ./$(@D)
	$(NETGO_CHECK)

test:
	go test ./...

integration-test: $(WEB_UPTODATE)
	docker-compose up -d
	docker run --rm \
		-v "$(PWD)":/usr/src/go/src/github.com/weaveworks/service/users \
		--workdir /usr/src/go/src/github.com/weaveworks/service/users \
		golang:1.4 \
		/bin/bash -c "go get -v -t ./... ; go test -tags integration -timeout 30s ./..."

clean:
	-docker rmi $(IMAGE_NAME)
	rm -rf $(USERS_EXE) $(IMAGE_TAR)
	go clean ./...
