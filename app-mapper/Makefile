.PHONY: all test integration-test clean

# If you can use Docker without being root, you can `make SUDO= <target>`
IMAGES_UPTODATE=.images.uptodate
APP_MAPPER_IMAGE=quay.io/weaveworks/app-mapper
# The DB image is only used in the local environemnt and it's not pushed to Quay
APP_MAPPER_DB_IMAGE=weaveworks/app-mapper-db
APP_MAPPER_EXE=app-mapper
SUDO=sudo
NETGO_CHECK=@strings $@ | grep cgo_stub\\\.go >/dev/null || { \
	rm $@; \
	echo "\nYour go standard library was built without the 'netgo' build tag."; \
	echo "To fix that, run"; \
	echo "    sudo go clean -i net"; \
	echo "    sudo go install -tags netgo std"; \
	false; \
}

all: $(IMAGES_UPTODATE)

$(IMAGES_UPTODATE): Dockerfile $(APP_MAPPER_EXE) db/Dockerfile db/schema.sql
	$(SUDO) docker build -t $(APP_MAPPER_IMAGE) .
	$(SUDO) docker build -t $(APP_MAPPER_DB_IMAGE) db
	touch $@

$(APP_MAPPER_EXE): *.go
	go get -d -tags netgo ./...
	go build -ldflags "-extldflags \"-static\" -linkmode=external" -tags netgo -o $@ ./$(@D)
	$(NETGO_CHECK)

integration-test: $(IMAGES_UPTODATE)
	script/integration-test.sh

test:
	go test ./...

clean:
	-$(SUDO) docker rmi $(APP_MAPPER_IMAGE) $(APP_MAPPER_DB_IMAGE) > /dev/null 2>&1 || true
	rm -rf $(APP_MAPPER_EXE) $(IMAGES_UPTODATE)
	go clean ./...
