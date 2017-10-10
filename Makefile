PHONY: all test notebooks-integration-test users-integration-test billing-integration-test clean images ui-upload
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
	$(SUDO) docker build -t $(IMAGE_PREFIX)/$(call image-suffix,$(@D)) $(@D)/
	$(SUDO) docker tag $(IMAGE_PREFIX)/$(call image-suffix,$(@D)) $(IMAGE_PREFIX)/$(call image-suffix,$(@D)):$(IMAGE_TAG)
	touch $@

# Get a list of directories containing Dockerfiles
DOCKERFILES=$(shell find * -type f -name Dockerfile ! -path "tools/*" ! -path "vendor/*")
UPTODATE_FILES=$(patsubst %/Dockerfile,%/$(UPTODATE),$(DOCKERFILES))
DOCKER_IMAGE_DIRS=$(patsubst %/Dockerfile,%,$(DOCKERFILES))
IMAGE_NAMES=$(foreach dir,$(DOCKER_IMAGE_DIRS),$(patsubst %,$(IMAGE_PREFIX)/%,$(call image-suffix,$(dir))))

images:
	$(info $(IMAGE_NAMES))

all: $(UPTODATE_FILES)

# Generating proto code is automated.
PROTO_DEFS := $(shell find . -type f -name "*.proto" ! -path "./tools/*" ! -path "./vendor/*")
PROTO_GOS := $(patsubst %.proto,%.pb.go,$(PROTO_DEFS))
users/users.pb.go: users/users.proto

MOCK_USERS := users/mock_users/usersclient.go
$(MOCK_USERS): users/users.pb.go

BILLING_DIR := billing
BILLING_TEST_DIRS := $(shell git ls-files -- '*_test.go' | grep -E "^$(BILLING_DIR)/" | xargs -n1 dirname | sort -u | sed -e 's|^|./|')

MOCK_BILLING_DB := $(BILLING_DIR)/db/mock_db/mock_db.go
MOCK_GOS := $(MOCK_USERS) $(MOCK_BILLING_DB)

# copy billing migrations into each billing application's directory
BILLING_MIGRATION_FILES := $(shell find $(BILLING_DIR)/db/migrations -type f)
$(BILLING_DIR)/aggregator/migrations/%: $(BILLING_DIR)/db/migrations/%
	mkdir -p $(@D)
	cp $< $@

$(BILLING_DIR)/api/migrations/%: $(BILLING_DIR)/db/migrations/%
	mkdir -p $(@D)
	cp $< $@

$(BILLING_DIR)/uploader/migrations/%: $(BILLING_DIR)/db/migrations/%
	mkdir -p $(@D)
	cp $< $@

# List of exes please
AUTHFE_EXE := authfe/authfe
USERS_EXE := users/cmd/users/users
METRICS_EXE := metrics/metrics
NOTEBOOKS_EXE := notebooks/cmd/notebooks/notebooks
SERVICE_UI_KICKER_EXE := service-ui-kicker/service-ui-kicker
GITHUB_RECEIVER_EXE := github-receiver/github-receiver
BILLING_EXE := $(BILLING_DIR)/api/api $(BILLING_DIR)/uploader/uploader $(BILLING_DIR)/aggregator/aggregator $(BILLING_DIR)/enforcer/enforcer
EXES = $(AUTHFE_EXE) $(USERS_EXE) $(METRICS_EXE) $(NOTEBOOKS_EXE) $(SERVICE_UI_KICKER_EXE) $(GITHUB_RECEIVER_EXE) $(BILLING_EXE)

# And what goes into each exe
COMMON := $(shell find common -name '*.go')
$(AUTHFE_EXE): $(shell find authfe -name '*.go') $(shell find users/client -name '*.go') $(COMMON) users/users.pb.go
$(USERS_EXE): $(shell find users -name '*.go') $(COMMON) users/users.pb.go
$(METRICS_EXE): $(shell find metrics -name '*.go') $(COMMON)
$(NOTEBOOKS_EXE): $(shell find notebooks -name '*.go') $(COMMON)
$(SERVICE_UI_KICKER_EXE): $(shell find service-ui-kicker -name '*.go') $(COMMON)
$(GITHUB_RECEIVER_EXE): $(shell find github-receiver -name '*.go') $(COMMON)
$(BILLING_EXE): $(shell find $(BILLING_DIR) -name '*.go') users/users.pb.go
test: users/users.pb.go

# And now what goes into each image
authfe/$(UPTODATE): $(AUTHFE_EXE)
users/$(UPTODATE): $(USERS_EXE) $(shell find users -name '*.sql') users/templates/*
metrics/$(UPTODATE): $(METRICS_EXE)
logging/$(UPTODATE): logging/fluent.conf logging/fluent-dev.conf logging/schema_service_events.json
build/$(UPTODATE): build/build.sh
notebooks/$(UPTODATE): $(NOTEBOOKS_EXE)
service-ui-kicker/$(UPTODATE): $(SERVICE_UI_KICKER_EXE)
github-receiver/$(UPTODATE): $(GITHUB_RECEIVER_EXE)

$(BILLING_DIR)/uploader/$(UPTODATE): $(patsubst $(BILLING_DIR)/db/migrations/%,$(BILLING_DIR)/uploader/migrations/%,$(BILLING_MIGRATION_FILES)) $(BILLING_DIR)/uploader/uploader
$(BILLING_DIR)/api/$(UPTODATE): $(patsubst $(BILLING_DIR)/db/migrations/%,$(BILLING_DIR)/api/migrations/%,$(BILLING_MIGRATION_FILES)) $(BILLING_DIR)/api/api
$(BILLING_DIR)/aggregator/$(UPTODATE): $(patsubst $(BILLING_DIR)/db/migrations/%,$(BILLING_DIR)/aggregator/migrations/%,$(BILLING_MIGRATION_FILES)) $(BILLING_DIR)/aggregator/aggregator
$(BILLING_DIR)/enforcer/$(UPTODATE): $(BILLING_DIR)/enforcer/enforcer

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

$(PROTO_GOS) $(MOCK_GOS) lint: build/$(UPTODATE)
	@mkdir -p $(shell pwd)/.pkg
	$(SUDO) docker run $(RM) -ti \
		-v $(shell pwd)/.pkg:/go/pkg \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		-e CIRCLECI -e CIRCLE_BUILD_NUM -e CIRCLE_NODE_TOTAL -e CIRCLE_NODE_INDEX -e COVERDIR \
		$(IMAGE_PREFIX)/build $@

$(EXES) test: build/$(UPTODATE) users/users.pb.go
	@mkdir -p $(shell pwd)/.pkg
	$(SUDO) docker run $(RM) -ti \
		-v $(shell pwd)/.pkg:/go/pkg \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		-e CIRCLECI -e CIRCLE_BUILD_NUM -e CIRCLE_NODE_TOTAL -e CIRCLE_NODE_INDEX -e COVERDIR \
		-e ZUORA_USERNAME=$(ZUORA_USERNAME) -e ZUORA_PASSWORD=$(ZUORA_PASSWORD) -e ZUORA_SUBSCRIPTIONPLANID=$(ZUORA_SUBSCRIPTIONPLANID) \
		$(IMAGE_PREFIX)/build $@

billing-integration-test: build/$(UPTODATE)
	@mkdir -p $(shell pwd)/.pkg
	DB_CONTAINER="$$(docker run -d -e 'POSTGRES_DB=billing_test' postgres:9.5)"; \
	$(SUDO) docker run $(RM) -ti \
		-v $(shell pwd)/.pkg:/go/pkg \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		-v $(shell pwd)/billing/db/migrations:/migrations \
		--workdir /go/src/github.com/weaveworks/service \
		--link "$$DB_CONTAINER":billing-db.weave.local \
		$(IMAGE_PREFIX)/build $@; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$DB_CONTAINER"; \
	exit $$status

else

$(EXES): build/$(UPTODATE) users/users.pb.go
	go build $(GO_FLAGS) -o $@ ./$(@D)
	$(NETGO_CHECK)

%.pb.go: build/$(UPTODATE)
	protoc -I ./vendor:./$(@D) --gogoslick_out=plugins=grpc:./$(@D) ./$(patsubst %.pb.go,%.proto,$@)

lint: build/$(UPTODATE)
	./tools/lint .

test: build/$(UPTODATE) users/users.pb.go $(MOCK_GOS)
	./tools/test -netgo -no-race

$(MOCK_USERS): build/$(UPTODATE)
	mockgen -destination $@ github.com/weaveworks/service/users UsersClient \
		&& sed -i'' s,github.com/weaveworks/service/vendor/,, $@

$(MOCK_BILLING_DB): build/$(UPTODATE) $(BILLING_DIR)/db/db.go
	mockgen -destination=$@ github.com/weaveworks/service/$(BILLING_DIR)/db DB

billing-integration-test: build/$(UPTODATE) $(MOCK_GOS)
	/bin/bash -c "go test -tags 'netgo integration' -timeout 30s $(BILLING_TEST_DIRS)"

endif


# Test and misc stuff
notebooks-integration-test: $(NOTEBOOKS_UPTODATE)
	DB_CONTAINER="$$(docker run -d -e 'POSTGRES_DB=notebooks_test' postgres:9.5)"; \
	docker run $(RM) \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		-v $(shell pwd)/notebooks/db/migrations:/migrations \
		--workdir /go/src/github.com/weaveworks/service/notebooks \
		--link "$$DB_CONTAINER":configs-db.weave.local \
		golang:1.8.3-stretch \
		/bin/bash -c "go test -tags integration -timeout 30s ./..."; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$DB_CONTAINER"; \
	exit $$status

users-integration-test: $(USERS_UPTODATE) users/users.pb.go
	DB_CONTAINER="$$(docker run -d -e 'POSTGRES_DB=users_test' postgres:9.5)"; \
	docker run $(RM) \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		-v $(shell pwd)/users/db/migrations:/migrations \
		--workdir /go/src/github.com/weaveworks/service/users \
		--link "$$DB_CONTAINER":users-db.weave.local \
		golang:1.8.3-stretch \
		/bin/bash -c "go test -tags integration -timeout 30s ./..."; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$DB_CONTAINER"; \
	exit $$status

clean:
	$(SUDO) docker rmi $(IMAGE_NAMES) >/dev/null 2>&1 || true
	rm -rf $(UPTODATE_FILES) $(EXES)
	rm -f users/users.pb.go
	rm -rf $(BILLING_DIR)/aggregator/migrations $(BILLING_DIR)/api/migrations $(BILLING_DIR)/uploader/migrations

	go clean ./...

# The following function will add `billing-` to the image name if part of billing/ dir
image-suffix = $(if $(filter billing%,$(1)),billing-$(shell basename $(1)),$(shell basename $(1)))
