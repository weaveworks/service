.PHONY: all test integration-test clean

IMAGES_UPTODATE=.images.uptodate
IMAGE_NAME=quay.io/weaveworks/users
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
DB_IMAGE_NAME=quay.io/weaveworks/users-db

all: $(IMAGES_UPTODATE)

$(IMAGES_UPTODATE): Dockerfile templates/* $(USERS_EXE) db/* ca-certificates.crt
	docker build -t $(IMAGE_NAME) .
	docker build -t $(DB_IMAGE_NAME) db/
	touch $@

ca-certificates.crt: /etc/ssl/certs/ca-certificates.crt
	cp $? $@

$(USERS_EXE): *.go names/*.go
	go get -d -tags netgo ./...
	go build -tags netgo -ldflags "-extldflags \"-static\" -linkmode=external" -o $@ ./$(@D)
	$(NETGO_CHECK)

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
	-docker rmi $(IMAGE_NAME) $(DB_IMAGE_NAME) > /dev/null 2>&1 || true
	rm -rf $(USERS_EXE) $(IMAGES_UPTODATE)
	go clean ./...
