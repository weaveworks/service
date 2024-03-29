.PHONY: all test generated \
	notebooks-integration-test users-integration-test billing-integration-test pubsub-integration-test \
	notification-integration-test flux-integration-test clean images ui-upload
.DEFAULT_GOAL := all

# Boiler plate for bulding Docker containers.
# All this must go at top of file I'm afraid.
IMAGE_PREFIX := weaveworks
IMAGE_TAG := $(shell ./tools/image-tag)
GIT_REVISION := $(shell git rev-parse HEAD)
UPTODATE := .uptodate
GO_TEST_IMAGE := golang:1.17-bullseye

# Building Docker images is now automated. The convention is every directory
# with a Dockerfile in it builds an image calls weaveworks/<dirname>.
# Dependencies (i.e. things that go in the image) still need to be explicitly
# declared.
%/$(UPTODATE): %/Dockerfile
	$(SUDO) docker build --build-arg=revision=$(GIT_REVISION) -t $(IMAGE_PREFIX)/$(shell basename $(@D)) $(@D)/
	$(SUDO) docker tag $(IMAGE_PREFIX)/$(shell basename $(@D)) $(IMAGE_PREFIX)/$(shell basename $(@D)):$(IMAGE_TAG)
	touch $@

# Get a list of directories containing Dockerfiles
DOCKERFILES=$(shell find * -type f -name Dockerfile ! -path "tools/*" ! -path "vendor/*")
UPTODATE_FILES=$(patsubst %/Dockerfile,%/$(UPTODATE),$(DOCKERFILES))
DOCKER_IMAGE_DIRS=$(patsubst %/Dockerfile,%,$(DOCKERFILES))
IMAGE_NAMES=$(foreach dir,$(DOCKER_IMAGE_DIRS),$(patsubst %,$(IMAGE_PREFIX)/%,$(shell basename $(dir))))

images:
	$(info $(IMAGE_NAMES))
	@echo > /dev/null

all: $(UPTODATE_FILES)

# Generating proto code is automated.
PROTO_DEFS := $(shell find . -type f -name "*.proto" ! -path "./tools/*" ! -path "./vendor/*")
define PROTODEF_template
 $(patsubst %.proto,%.pb.go, $(1)): $(1)
endef
$(foreach def,$(PROTO_DEFS),$(eval $(call PROTODEF_template,$(def))))
PROTO_GOS := $(patsubst %.proto,%.pb.go,$(PROTO_DEFS))

MOCK_USERS := users/mock_users/mock_usersclient.go
$(MOCK_USERS): users/users.pb.go

# copied from tools/test, but additionally excluding e2e dirs
TESTDIRS := "$(shell git ls-files -- '*_test.go' | grep -vE '^(vendor|experimental|.*\/e2e)/' | xargs -n1 dirname | sort -u | sed -e 's|^|./|')"

BILLING_DB := billing-api/db
BILLING_TEST_DIRS := $(shell find . -name '*_test.go' | grep -E  "^\./billing" | xargs -n1 dirname | sort -u)

MOCK_BILLING_DB := $(BILLING_DB)/mock_db/mock_db.go

BILLING_GRPC := common/billing/grpc/billing.pb.go
MOCK_BILLING_GRPC := common/billing/grpc/mock_grpc.go
# Mocks can only be generated once protoc has generated the *.pb.go file:
$(MOCK_BILLING_GRPC): $(BILLING_GRPC)

MOCK_COMMON_GCP_PROCUREMENT_CLIENT := common/gcp/procurement/mock_procurement/mock_client.go

MOCK_GOS := $(MOCK_USERS) $(MOCK_BILLING_DB) $(MOCK_BILLING_GRPC) $(MOCK_COMMON_GCP_PROCUREMENT_CLIENT)

# copy billing migrations into each billing application's directory
billing-aggregator/migrations/%: $(BILLING_DB)/migrations/%
	mkdir -p $(@D)
	cp $< $@

billing-uploader/migrations/%: $(BILLING_DB)/migrations/%
	mkdir -p $(@D)
	cp $< $@

BILLING_MIGRATION_FILES := $(shell find $(BILLING_DB)/migrations -type f)
billing-migrations-deps = $(patsubst $(BILLING_DB)/migrations/%,$(1)/migrations/%,$(BILLING_MIGRATION_FILES))

# common templates and services which depend on common templates
COMMON_TEMPLATE_FILES := $(shell find common/templates -type f)
common-templates-deps = $(patsubst common/templates/%,$(1)/templates/%,$(COMMON_TEMPLATE_FILES))

users/templates/%: common/templates/%
	cp $< $@

notification-eventmanager/templates/%: common/templates/%
	cp $< $@

### BEGIN: Msgpack code generation for billing-synthetic-usage-injector.
### N.B.: This typically should replicate what is in github.com/weaveworks/scope/tree/master/Makefile
GO_ENV=GOGC=off
GOOS=$(shell go tool dist env | grep GOOS | sed -e 's/GOOS="\(.*\)"/\1/')
ifeq ($(GOOS),linux)
GO_ENV+=CGO_ENABLED=1
endif
NO_CROSS_COMP=unset GOOS GOARCH
GO_HOST=$(NO_CROSS_COMP); env $(GO_ENV) go
WITH_GO_HOST_ENV=$(NO_CROSS_COMP); $(GO_ENV)

GO_BUILD_INSTALL_DEPS=-i
GO_BUILD_TAGS='netgo unsafe'
GO_BUILD_FLAGS=$(GO_BUILD_INSTALL_DEPS) -ldflags "-extldflags \"-static\" -X main.version=$(GIT_REVISION) -s -w" -tags $(GO_BUILD_TAGS)

CODECGEN_TARGETS=vendor/github.com/weaveworks/scope/report/report.codecgen.go
CODECGEN_DIR=vendor/github.com/ugorji/go/codec/codecgen
CODECGEN_BIN_DIR=$(CODECGEN_DIR)/bin
CODECGEN_EXE=$(CODECGEN_BIN_DIR)/codecgen

CODECGEN_UID=0
GET_CODECGEN_DEPS=$(shell find $(1) -maxdepth 1 -type f -name '*.go' -not -name '*_test.go' -not -name '*.codecgen.go' -not -name '*.generated.go')
### END: Msgpack code generation for billing-synthetic-usage-injector

flux-api/migrations.tar:
	tar cf $@ flux-api/db/migrations

# List of exes please
AUTHFE_EXE := authfe/authfe
USERS_EXE := users/cmd/users/users
USERS_SYNC_EXE := users-sync/cmd/users-sync
METRICS_EXE := metrics/metrics
METRICS_USAGE_EXE := metrics-usage/metrics-usage
NOTEBOOKS_EXE := notebooks/cmd/notebooks/notebooks
SERVICE_UI_KICKER_EXE := service-ui-kicker/service-ui-kicker
FLUX_API_EXE := flux-api/flux-api
BILLING_USAGE_INJECTOR_EXE := billing-synthetic-usage-injector/injector
BILLING_EXES := billing-api/billing-api billing-uploader/uploader billing-aggregator/aggregator billing-enforcer/enforcer $(BILLING_USAGE_INJECTOR_EXE)
GCP_LAUNCHER_WEBHOOK_EXE := gcp-launcher-webhook/gcp-launcher-webhook
KUBECTL_SERVICE_EXE := kubectl-service/kubectl-service
GCP_SERVICE_EXE := gcp-service/gcp-service
NOTIFICATION_EXES := notification-eventmanager/cmd/eventmanager/eventmanager notification-sender/cmd/sender/sender
DASHBOARD_EXE := dashboard-api/dashboard-api
NET_DISCOVERY_EXE := service-net-discovery/cmd/discovery/discovery
SCOPE_DATA_CLEANING_EXE := scope-data-cleaning/scanner
EXES = $(AUTHFE_EXE) $(USERS_EXE) $(USERS_SYNC_EXE) $(METRICS_EXE) $(NOTEBOOKS_EXE) $(SERVICE_UI_KICKER_EXE) \
	$(GITHUB_RECEIVER_EXE) $(FLUX_API_EXE) $(BILLING_EXES) $(GCP_LAUNCHER_WEBHOOK_EXE) \
	$(NOTIFICATION_EXES) $(KUBECTL_SERVICE_EXE) $(GCP_SERVICE_EXE) $(DASHBOARD_EXE) $(NET_DISCOVERY_EXE) \
	$(SCOPE_DATA_CLEANING_EXE) $(METRICS_USAGE_EXE)

# And what goes into each exe
gofiles = $(shell find $1 -name '*.go')
basedir = $(firstword $(subst /, ,$1))
COMMON := $(call gofiles,common)
$(AUTHFE_EXE): $(call gofiles,authfe) $(call gofiles,users/client) $(COMMON) users/users.pb.go
$(USERS_EXE): $(call gofiles,users) $(COMMON) users/users.pb.go
$(USERS_SYNC_EXE): $(call gofiles,users-sync) $(COMMON) users-sync/api/users-sync.pb.go
$(METRICS_EXE): $(call gofiles,metrics) $(COMMON)
$(METRICS_USAGE_EXE): $(call gofiles,metrics-usage) $(COMMON)
$(NOTEBOOKS_EXE): $(call gofiles,notebooks) $(COMMON)
$(SERVICE_UI_KICKER_EXE): $(call gofiles,service-ui-kicker) $(COMMON)
$(FLUX_API_EXE): $(call gofiles,flux-api) $(COMMON)
$(GCP_LAUNCHER_WEBHOOK_EXE): $(call gofiles,gcp-launcher-webhook) $(COMMON)
$(KUBECTL_SERVICE_EXE): $(shell find kubectl-service -name '*.go') $(COMMON)
$(GCP_SERVICE_EXE): $(shell find gcp-service -name '*.go') $(COMMON) $(KUBECTL_SERVICE_EXE)
$(NOTIFICATION_EXES): $(call gofiles,notification-*) $(COMMON)
$(DASHBOARD_EXE): $(call gofiles,dashboard-*) $(COMMON)
$(BILLING_USAGE_INJECTOR_EXE): $(CODECGEN_TARGETS)
$(NET_DISCOVERY_EXE): $(call gofiles,service-net-discovery) $(COMMON)
$(SCOPE_DATA_CLEANING_EXE): $(call gofiles,scope-data-cleaning)
# See secondary expansion at bottom for BILLING_EXES gofiles

test: $(PROTO_GOS)

# And now what goes into each image
authfe/$(UPTODATE): $(AUTHFE_EXE)
users/$(UPTODATE): $(USERS_EXE) $(shell find users -name '*.sql') $(call common-templates-deps,users) users/templates/*
users-sync/$(UPTODATE): $(USERS_SYNC_EXE)
metrics/$(UPTODATE): $(METRICS_EXE)
metrics-usage/$(UPTODATE): $(METRICS_USAGE_EXE)
logging/$(UPTODATE): logging/fluent.conf logging/fluent-dev.conf logging/schema_service_events.json
build/$(UPTODATE): build/build.sh
notebooks/$(UPTODATE): $(NOTEBOOKS_EXE)
service-ui-kicker/$(UPTODATE): $(SERVICE_UI_KICKER_EXE)
flux-api/$(UPTODATE): $(FLUX_API_EXE) flux-api/migrations.tar
gcp-launcher-webhook/$(UPTODATE): $(GCP_LAUNCHER_WEBHOOK_EXE)
kubectl-service/$(UPTODATE): $(KUBECTL_SERVICE_EXE)
gcp-service/$(UPTODATE): $(GCP_SERVICE_EXE)
dashboard-api/$(UPTODATE): $(DASHBOARD_EXE)
billing-exporter/$(UPTODATE): $(shell find billing-exporter -name '*.py') billing-exporter/Pipfile billing-exporter/Pipfile.lock
service-net-discovery/$(UPTODATE): $(NET_DISCOVERY_EXE)
scope-data-cleaning/$(UPTODATE): $(SCOPE_DATA_CLEANING_EXE)

# Expands a list of binary paths to have their respective images depend on the binary
# Example:
#   $(eval $(call IMAGEDEP_template,"foo/cmd/foo bar/cmd/bar"))
# Output:
# foo/$(UPTODATE): foo/cmd/foo
# bar/$(UPTODATE): bar/cmd/bar
define IMAGEDEP_template
 $(call basedir,$(1))/$$(UPTODATE): $(1)
endef

$(foreach exe,$(BILLING_EXES),$(eval $(call IMAGEDEP_template,$(exe))))
billing-uploader/$(UPTODATE): $(call billing-migrations-deps,billing-uploader)
billing-aggregator/$(UPTODATE): $(call billing-migrations-deps,billing-aggregator)

$(foreach nexe,$(NOTIFICATION_EXES),$(eval $(call IMAGEDEP_template,$(nexe))))
notification-eventmanager/$(UPTODATE): $(NOTIFICATION_EXES) $(wildcard notification-eventmanager/migrations/*) $(call common-templates-deps,notification-eventmanager) notification-eventmanager/templates/*

# All the boiler plate for building golang follows:
SUDO := $(shell docker info >/dev/null 2>&1 || echo "sudo -E")
BUILD_IN_CONTAINER := true
RM := --rm
GO_FLAGS := -ldflags "-extldflags \"-static\" -s -w" -tags netgo -i
NETGO_CHECK = @strings $@ | grep cgo_stub\\\.go >/dev/null || { \
	rm $@; \
	echo "\nYour go standard library was built without the 'netgo' build tag."; \
	echo "To fix that, run"; \
	echo "    sudo go clean -i net"; \
	echo "    sudo go install -tags netgo std"; \
	false; \
}

ifeq ($(BUILD_IN_CONTAINER),true)

$(CODECGEN_EXE): build/$(UPTODATE)
	mkdir -p $(@D)
	$(SUDO) docker run $(RM) -ti \
		-v $(shell pwd)/.pkg:/go/pkg \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		-e CIRCLECI -e CIRCLE_BUILD_NUM -e CIRCLE_NODE_TOTAL -e CIRCLE_NODE_INDEX -e COVERDIR \
		$(IMAGE_PREFIX)/build $@

$(CODECGEN_TARGETS): $(CODECGEN_EXE) $(call GET_CODECGEN_DEPS,vendor/github.com/weaveworks/scope/report/)
	$(SUDO) docker run $(RM) -ti \
		-v $(shell pwd)/.pkg:/go/pkg \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		-e CIRCLECI -e CIRCLE_BUILD_NUM -e CIRCLE_NODE_TOTAL -e CIRCLE_NODE_INDEX -e COVERDIR \
		$(IMAGE_PREFIX)/build $@

$(PROTO_GOS) $(MOCK_GOS) generated lint: build/$(UPTODATE)
	@mkdir -p $(shell pwd)/.pkg
	$(SUDO) docker run $(RM) -i \
		-v $(shell pwd)/.pkg:/go/pkg \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		-e CIRCLECI -e CIRCLE_BUILD_NUM -e CIRCLE_NODE_TOTAL -e CIRCLE_NODE_INDEX -e COVERDIR \
		$(IMAGE_PREFIX)/build $@

$(EXES) test: build/$(UPTODATE) $(PROTO_GOS)
	@mkdir -p $(shell pwd)/.pkg
	$(SUDO) docker run $(RM) -i \
		-v $(shell pwd)/.pkg:/go/pkg \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		-e CIRCLECI -e CIRCLE_BUILD_NUM -e CIRCLE_NODE_TOTAL -e CIRCLE_NODE_INDEX -e COVERDIR \
		-e TESTDIRS=${TESTDIRS} \
		$(IMAGE_PREFIX)/build $@

flux-integration-test: build/$(UPTODATE)
	@mkdir -p $(shell pwd)/.pkg
	NATS_CONTAINER="$$(docker run -d nats)"; \
	POSTGRES_CONTAINER="$$(docker run -d postgres:10.6)"; \
	$(SUDO) docker run $(RM) -ti \
		-v $(shell pwd)/.pkg:/go/pkg \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		-v $(shell pwd)/billing-api/db/migrations:/migrations \
		--workdir /go/src/github.com/weaveworks/service \
		--link "$$NATS_CONTAINER":nats \
		--link "$$POSTGRES_CONTAINER":postgres \
		$(IMAGE_PREFIX)/build $@; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$NATS_CONTAINER" "$$POSTGRES_CONTAINER"; \
	exit $$status

else
$(CODECGEN_EXE): build/$(UPTODATE) $(CODECGEN_DIR)/*.go
	go build $(GO_FLAGS) -o $@ ./$(CODECGEN_DIR)

$(CODECGEN_TARGETS): $(CODECGEN_EXE) $(call GET_CODECGEN_DEPS,vendor/github.com/weaveworks/scope/report/)
	rm -f $@; go build $(GO_FLAGS) ./$(@D) # workaround for https://github.com/ugorji/go/issues/145
	cd $(@D) && $(WITH_GO_HOST_ENV) $(shell pwd)/$(CODECGEN_EXE) -d $(CODECGEN_UID) -rt $(GO_BUILD_TAGS) -u -o $(@F) $(notdir $(call GET_CODECGEN_DEPS,$(@D)))

$(EXES): build/$(UPTODATE) $(PROTO_GOS)
	CGO_ENABLED=0 go build $(GO_FLAGS) -o $@ ./$(@D)
	$(NETGO_CHECK)

generated: $(PROTO_GOS) $(MOCK_GOS)

%.pb.go: build/$(UPTODATE)
	protoc -I ./vendor:./$(@D) --gogoslick_out=plugins=grpc:./$(@D) ./$(patsubst %.pb.go,%.proto,$@)

lint: build/$(UPTODATE) $(PROTO_GOS) $(MOCK_GOS)
	./tools/lint .

test: build/$(UPTODATE) $(PROTO_GOS) $(MOCK_GOS) $(CODECGEN_TARGETS)
	TESTDIRS=${TESTDIRS} NO_SCHEDULER="true" ./tools/test -netgo -no-race

$(MOCK_USERS): build/$(UPTODATE)
	mockgen -destination=$@ github.com/weaveworks/service/users UsersClient \
		&& sed -i'' s,github.com/weaveworks/service/vendor/,, $@

$(MOCK_BILLING_DB): build/$(UPTODATE) $(BILLING_DB)/db.go
	mockgen -destination=$@ github.com/weaveworks/service/$(BILLING_DB) DB

$(MOCK_BILLING_GRPC): build/$(UPTODATE)
	mockgen -source=$(BILLING_GRPC) -destination=$@ -package=grpc

$(MOCK_COMMON_GCP_PROCUREMENT_CLIENT): build/$(UPTODATE)
	mockgen -destination=$@ github.com/weaveworks/service/common/gcp/procurement API \
		&& sed -i'' s,github.com/weaveworks/service/vendor/,, $@

billing-integration-test: build/$(UPTODATE) $(MOCK_GOS) $(CODECGEN_TARGETS)
	/bin/bash -c "go test -tags 'netgo integration' -timeout 2m $(BILLING_TEST_DIRS)"

flux-integration-test:
# These packages must currently be tested in series because
# otherwise they will all race to run migrations.
	/bin/bash -c "go test -tags integration -timeout 30s ./flux-api"
	/bin/bash -c "go test -tags integration -timeout 30s ./flux-api/bus/nats"
	/bin/bash -c "go test -tags integration -timeout 30s ./flux-api/history/sql"
	/bin/bash -c "go test -tags integration -timeout 30s ./flux-api/instance/sql"

endif


# Test and misc stuff
notebooks-integration-test: $(NOTEBOOKS_UPTODATE)
	DB_CONTAINER="$$(docker run -d -e 'POSTGRES_DB=notebooks_test' postgres:10.6)"; \
	docker run $(RM) \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		-v $(shell pwd)/notebooks/db/migrations:/migrations \
		--workdir /go/src/github.com/weaveworks/service/notebooks \
		--link "$$DB_CONTAINER":configs-db.weave.local \
		$(GO_TEST_IMAGE) \
		/bin/bash -c "GO111MODULE=off go test -tags integration -timeout 30s ./..."; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$DB_CONTAINER"; \
	exit $$status

users-integration-test: $(USERS_UPTODATE) $(PROTO_GOS) $(MOCK_GOS)
	DB_CONTAINER="$$(docker run -d -e 'POSTGRES_DB=users_test' postgres:10.6)"; \
	docker run $(RM) \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		-v $(shell pwd)/users/db/migrations:/migrations \
		--workdir /go/src/github.com/weaveworks/service/users \
		--link "$$DB_CONTAINER":users-db.weave.local \
		$(GO_TEST_IMAGE) \
		/bin/bash -c "GO111MODULE=off go test -tags integration -timeout 30s ./..."; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$DB_CONTAINER"; \
	exit $$status

pubsub-integration-test:
	PUBSUB_EMU_CONTAINER="$$(docker run --net=host -p 127.0.0.1:8085:8085 -d adilsoncarvalho/gcloud-pubsub-emulator:latest)"; \
	docker run $(RM) \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		--net=host -p 127.0.0.1:1337:1337 \
		--workdir /go/src/github.com/weaveworks/service/common/gcp/pubsub \
		$(GO_TEST_IMAGE) \
		/bin/bash -c "GO111MODULE=off RUN_MANUAL_TEST=1 go test -tags integration -timeout 30s ./..."; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$PUBSUB_EMU_CONTAINER"; \
	exit $$status

kubectl-service-integration-test: kubectl-service/$(UPTODATE) kubectl-service/grpc/kubectl-service.pb.go
	SVC_CONTAINER="$$(docker run -d -p 4887:4772 -p 8887:80 $(IMAGE_PREFIX)/kubectl-service -dry-run=true)"; \
	docker run $(RM) \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		--workdir /go/src/github.com/weaveworks/service/kubectl-service \
		--link "$$SVC_CONTAINER":kubectl-service.weave.local \
		$(GO_TEST_IMAGE) \
		/bin/bash -c "GO111MODULE=off go test -tags integration -timeout 30s ./..."; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$SVC_CONTAINER"; \
	exit $$status

gcp-service-integration-test: gcp-service/$(UPTODATE) gcp-service/grpc/gcp-service.pb.go
	SVC_CONTAINER="$$(docker run -d -p 4888:4772 -p 8888:80 $(IMAGE_PREFIX)/gcp-service -dry-run=true)"; \
	docker run $(RM) \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		--workdir /go/src/github.com/weaveworks/service/gcp-service \
		--link "$$SVC_CONTAINER":gcp-service.weave.local \
		$(GO_TEST_IMAGE) \
		/bin/bash -c "GO111MODULE=off go test -tags integration -timeout 30s ./..."; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$SVC_CONTAINER"; \
	exit $$status

notification-integration-test: notification-eventmanager/$(UPTODATE) notification-sender/$(UPTODATE)
	cd notification-eventmanager/e2e && $(SUDO) docker-compose up --abort-on-container-exit; EXIT_CODE=$$?; $(SUDO) docker-compose down; exit $$EXIT_CODE

clean:
	$(SUDO) docker rmi $(IMAGE_NAMES) >/dev/null 2>&1 || true
	rm -rf $(UPTODATE_FILES) $(EXES)
	rm -fr $(CODECGEN_TARGETS) $(CODECGEN_BIN_DIR)
	rm -rf billing-aggregator/migrations billing-uploader/migrations
	rm -f $(call common-templates-deps,users) $(call common-templates-deps,notification-eventmanager)
	go clean ./...

# For .SECONDEXPANSION docs, see https://www.gnu.org/software/make/manual/html_node/Special-Targets.html
.SECONDEXPANSION:
$(BILLING_EXES): $$(shell find $$(@D) -name '*.go') $(COMMON) $(shell find $(BILLING_DB) -name '*.go') users/users.pb.go
