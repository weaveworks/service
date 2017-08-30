.PHONY: all test notebooks-integration-test users-integration-test clean client-lint images ui-upload
.DEFAULT_GOAL := all

# Boiler plate for bulding Docker containers.
# All this must go at top of file I'm afraid.
IMAGE_PREFIX := quay.io/weaveworks
IMAGE_TAG := $(shell ./tools/image-tag)
UPTODATE := .uptodate

# Building Docker images is now automated. The convention is every directory
# with a Dockerfile in it builds an image calls quay.io/weaveworks/<dirname>.
# Dependencies (i.e. things that go in the image) still need to be explicitly
# declared.
%/$(UPTODATE): %/Dockerfile
	$(SUDO) docker build -t $(IMAGE_PREFIX)/$(shell basename $(@D)) $(@D)/
	$(SUDO) docker tag $(IMAGE_PREFIX)/$(shell basename $(@D)) $(IMAGE_PREFIX)/$(shell basename $(@D)):$(IMAGE_TAG)
	touch $@

# Get a list of directories containing Dockerfiles
DOCKERFILES=$(shell find * -type f -name Dockerfile ! -path "tools/*" ! -path "vendor/*")
UPTODATE_FILES=$(patsubst %/Dockerfile,%/$(UPTODATE),$(DOCKERFILES))
DOCKER_IMAGE_DIRS=$(patsubst %/Dockerfile,%,$(DOCKERFILES))
IMAGE_NAMES=$(foreach dir,$(DOCKER_IMAGE_DIRS),$(patsubst %,$(IMAGE_PREFIX)/%,$(shell basename $(dir))))

images:
	$(info $(IMAGE_NAMES))

all: $(UPTODATE_FILES)

# Generating proto code is automated.
PROTO_DEFS := $(shell find . -type f -name "*.proto" ! -path "./tools/*" ! -path "./vendor/*")
PROTO_GOS := $(patsubst %.proto,%.pb.go,$(PROTO_DEFS))
users/users.pb.go: users/users.proto

# List of exes please
AUTHFE_EXE := authfe/authfe
USERS_EXE := users/cmd/users/users
METRICS_EXE := metrics/metrics
NOTEBOOKS_EXE := notebooks/cmd/notebooks/notebooks
SERVICE_UI_KICKER_EXE := service-ui-kicker/service-ui-kicker
EXES = $(AUTHFE_EXE) $(USERS_EXE) $(METRICS_EXE) $(NOTEBOOKS_EXE) $(SERVICE_UI_KICKER_EXE)

# And what goes into each exe
COMMON := $(shell find common -name '*.go')
$(AUTHFE_EXE): $(shell find authfe -name '*.go') $(shell find users/client -name '*.go') $(COMMON) users/users.pb.go
$(USERS_EXE): $(shell find users -name '*.go') $(COMMON) users/users.pb.go
$(METRICS_EXE): $(shell find metrics -name '*.go') $(COMMON)
$(NOTEBOOKS_EXE): $(shell find notebooks -name '*.go') $(COMMON)
$(SERVICE_UI_KICKER_EXE): $(shell find service-ui-kicker -name '*.go') $(COMMON)
test: users/users.pb.go

# And now what goes into each image
authfe/$(UPTODATE): $(AUTHFE_EXE)
users/$(UPTODATE): $(USERS_EXE) $(shell find users -name '*.sql') users/templates/*
metrics/$(UPTODATE): $(METRICS_EXE)
logging/$(UPTODATE): logging/fluent.conf logging/fluent-dev.conf logging/schema_service_events.json
build/$(UPTODATE): build/build.sh
notebooks/$(UPTODATE): $(NOTEBOOKS_EXE)
service-ui-kicker/$(UPTODATE): $(SERVICE_UI_KICKER_EXE)

# All the boiler plate for building golang follows:
SUDO := $(shell docker info >/dev/null 2>&1 || echo "sudo -E")
BUILD_IN_CONTAINER := true
RM := --rm
GO_FLAGS := -ldflags "-extldflags \"-static\" -linkmode=external -s -w" -tags netgo -i
NETGO_CHECK = @strings $@ | grep cgo_stub\\\.go >/dev/null || { \
	rm $@; \
	echo "\nYour go standard library was built without the 'netgo' build tag."; \
	echo "To fix that, run"; \
	echo "    sudo go clean -i net"; \
	echo "    sudo go install -tags netgo std"; \
	false; \
}

ifeq ($(BUILD_IN_CONTAINER),true)

$(EXES) $(PROTO_GOS) lint test: build/$(UPTODATE)
	@mkdir -p $(shell pwd)/.pkg
	$(SUDO) docker run $(RM) -ti \
		-v $(shell pwd)/.pkg:/go/pkg \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		-e CIRCLECI -e CIRCLE_BUILD_NUM -e CIRCLE_NODE_TOTAL -e CIRCLE_NODE_INDEX -e COVERDIR \
		$(IMAGE_PREFIX)/build $@

else

$(EXES): build/$(UPTODATE)
	go build $(GO_FLAGS) -o $@ ./$(@D)
	$(NETGO_CHECK)

%.pb.go: build/$(UPTODATE)
	protoc -I ./vendor:./$(@D) --gogoslick_out=plugins=grpc:./$(@D) ./$(patsubst %.pb.go,%.proto,$@)

lint: build/$(UPTODATE)
	./tools/lint .

test: build/$(UPTODATE)
	./tools/test -netgo -no-race

endif


# Test and misc stuff
notebooks-integration-test: $(NOTEBOOKS_UPTODATE)
	DB_CONTAINER="$$(docker run -d -e 'POSTGRES_DB=notebooks_test' postgres:9.4)"; \
	docker run $(RM) \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		-v $(shell pwd)/notebooks/db/migrations:/migrations \
		--workdir /go/src/github.com/weaveworks/service/notebooks \
		--link "$$DB_CONTAINER":configs-db.weave.local \
		golang:1.8.0 \
		/bin/bash -c "go test -tags integration -timeout 30s ./..."; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$DB_CONTAINER"; \
	exit $$status

users-integration-test: $(USERS_UPTODATE)
	DB_CONTAINER="$$(docker run -d -e 'POSTGRES_DB=users_test' postgres:9.4)"; \
	docker run $(RM) \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		-v $(shell pwd)/users/db/migrations:/migrations \
		--workdir /go/src/github.com/weaveworks/service/users \
		--link "$$DB_CONTAINER":users-db.weave.local \
		golang:1.8.0 \
		/bin/bash -c "go test -tags integration -timeout 30s ./..."; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$DB_CONTAINER"; \
	exit $$status

clean:
	$(SUDO) docker rmi $(IMAGE_NAMES) >/dev/null 2>&1 || true
	rm -rf $(UPTODATE_FILES) $(EXES)
	go clean ./...

