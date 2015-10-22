.PHONY: all test integration-test clean

RM=--rm
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
# The DB image is only used in the local environemnt and it's not pushed to Quay
DB_IMAGE_NAME=weaveworks/users-db

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

integration-test: $(IMAGES_UPTODATE)
	DB_CONTAINER="$$(docker run -d $(DB_IMAGE_NAME))"; \
	docker run $(RM) \
		-v "$(GOPATH)":/go/ \
		--workdir /go/src/github.com/weaveworks/service/users \
		--link "$$DB_CONTAINER":users-db.weave.local \
		golang:1.5.1 \
		/bin/bash -c "go get -v -d -t ./... ; go test -tags integration -timeout 30s ./..."; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$DB_CONTAINER"; \
	exit $$status

clean:
	-docker rmi $(IMAGE_NAME) $(DB_IMAGE_NAME) > /dev/null 2>&1 || true
	rm -rf $(USERS_EXE) $(IMAGES_UPTODATE)
	go clean ./...
