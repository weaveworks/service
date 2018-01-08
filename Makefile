.PHONY: all test \
	notebooks-integration-test users-integration-test billing-integration-test pubsub-integration-test \
	flux-nats-tests clean images ui-upload
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
	@echo > /dev/null

all: $(UPTODATE_FILES)

# Generating proto code is automated.
PROTO_DEFS := $(shell find . -type f -name "*.proto" ! -path "./tools/*" ! -path "./vendor/*")
PROTO_GOS := $(patsubst %.proto,%.pb.go,$(PROTO_DEFS))
users/users.pb.go: users/users.proto
kubectl-service/grpc/kubectl-service.pb.go: kubectl-service/grpc/kubectl-service.proto
gcp-service/grpc/gcp-service.pb.go: gcp-service/grpc/gcp-service.proto

MOCK_USERS := users/mock_users/mock_usersclient.go
$(MOCK_USERS): users/users.pb.go

BILLING_DB := billing-api/db
BILLING_TEST_DIRS := $(shell find . -name '*_test.go' | grep -E  "^\./billing" | xargs -n1 dirname | sort -u)

MOCK_BILLING_DB := $(BILLING_DB)/mock_db/mock_db.go
MOCK_COMMON_GCP_PARTNER_CLIENT := common/gcp/partner/mock_partner/mock_client.go
MOCK_COMMON_GCP_PARTNER_ACCESS := common/gcp/partner/mock_partner/mock_access.go

MOCK_GOS := $(MOCK_USERS) $(MOCK_BILLING_DB) $(MOCK_COMMON_GCP_PARTNER_CLIENT) $(MOCK_COMMON_GCP_PARTNER_ACCESS)


# copy billing migrations into each billing application's directory
billing-aggregator/migrations/%: $(BILLING_DB)/migrations/%
	mkdir -p $(@D)
	cp $< $@

billing-uploader/migrations/%: $(BILLING_DB)/migrations/%
	mkdir -p $(@D)
	cp $< $@

BILLING_MIGRATION_FILES := $(shell find $(BILLING_DB)/migrations -type f)
billing-migrations-deps = $(patsubst $(BILLING_DB)/migrations/%,$(1)/migrations/%,$(BILLING_MIGRATION_FILES))

flux-api/migrations.tar:
	tar cf $@ flux-api/db/migrations

# List of exes please
AUTHFE_EXE := authfe/authfe
USERS_EXE := users/cmd/users/users
METRICS_EXE := metrics/metrics
NOTEBOOKS_EXE := notebooks/cmd/notebooks/notebooks
SERVICE_UI_KICKER_EXE := service-ui-kicker/service-ui-kicker
GITHUB_RECEIVER_EXE := github-receiver/github-receiver
FLUX_API_EXE := flux-api/flux-api
BILLING_EXE := billing-api/api billing-uploader/uploader billing-aggregator/aggregator billing-enforcer/enforcer
GCP_LAUNCHER_WEBHOOK_EXE := gcp-launcher-webhook/gcp-launcher-webhook
KUBECTL_SERVICE_EXE := kubectl-service/kubectl-service
GCP_SERVICE_EXE := gcp-service/gcp-service
EXES = $(AUTHFE_EXE) $(USERS_EXE) $(METRICS_EXE) $(NOTEBOOKS_EXE) $(SERVICE_UI_KICKER_EXE) $(GITHUB_RECEIVER_EXE) $(FLUX_API_EXE) $(BILLING_EXE) $(GCP_LAUNCHER_WEBHOOK_EXE) $(KUBECTL_SERVICE_EXE) $(GCP_SERVICE_EXE)

# And what goes into each exe
COMMON := $(shell find common -name '*.go')
$(AUTHFE_EXE): $(shell find authfe -name '*.go') $(shell find users/client -name '*.go') $(COMMON) users/users.pb.go
$(USERS_EXE): $(shell find users -name '*.go') $(COMMON) users/users.pb.go
$(METRICS_EXE): $(shell find metrics -name '*.go') $(COMMON)
$(NOTEBOOKS_EXE): $(shell find notebooks -name '*.go') $(COMMON)
$(SERVICE_UI_KICKER_EXE): $(shell find service-ui-kicker -name '*.go') $(COMMON)
$(GITHUB_RECEIVER_EXE): $(shell find github-receiver -name '*.go') $(COMMON)
$(FLUX_API_EXE): $(shell find flux-api -name '*.go') $(COMMON)

$(GCP_LAUNCHER_WEBHOOK_EXE): $(shell find gcp-launcher-webhook -name '*.go') $(COMMON)
$(KUBECTL_SERVICE_EXE): $(shell find kubectl-service -name '*.go') $(COMMON)
$(GCP_SERVICE_EXE): $(shell find gcp-service -name '*.go') $(COMMON) $(KUBECTL_SERVICE_EXE)
test: users/users.pb.go kubectl-service/grpc/kubectl-service.pb.go gcp-service/grpc/gcp-service.pb.go

# And now what goes into each image
authfe/$(UPTODATE): $(AUTHFE_EXE)
users/$(UPTODATE): $(USERS_EXE) $(shell find users -name '*.sql') users/templates/*
metrics/$(UPTODATE): $(METRICS_EXE)
logging/$(UPTODATE): logging/fluent.conf logging/fluent-dev.conf logging/schema_service_events.json
build/$(UPTODATE): build/build.sh
notebooks/$(UPTODATE): $(NOTEBOOKS_EXE)
service-ui-kicker/$(UPTODATE): $(SERVICE_UI_KICKER_EXE)
github-receiver/$(UPTODATE): $(GITHUB_RECEIVER_EXE)
flux-api/$(UPTODATE): $(FLUX_API_EXE) flux-api/migrations.tar
gcp-launcher-webhook/$(UPTODATE): $(GCP_LAUNCHER_WEBHOOK_EXE)
kubectl-service/$(UPTODATE): $(KUBECTL_SERVICE_EXE)
gcp-service/$(UPTODATE): $(GCP_SERVICE_EXE)

billing-uploader/$(UPTODATE): billing-uploader/uploader $(call billing-migrations-deps,billing-uploader)
billing-aggregator/$(UPTODATE): billing-aggregator/aggregator $(call billing-migrations-deps,billing-aggregator)
billing-api/$(UPTODATE): billing-api/api
billing-enforcer/$(UPTODATE): billing-enforcer/enforcer

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

$(EXES) test: build/$(UPTODATE) users/users.pb.go kubectl-service/grpc/kubectl-service.pb.go gcp-service/grpc/gcp-service.pb.go
	@mkdir -p $(shell pwd)/.pkg
	$(SUDO) docker run $(RM) -ti \
		-v $(shell pwd)/.pkg:/go/pkg \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		-e CIRCLECI -e CIRCLE_BUILD_NUM -e CIRCLE_NODE_TOTAL -e CIRCLE_NODE_INDEX -e COVERDIR \
		-e ZUORA_USERNAME=$(ZUORA_USERNAME) -e ZUORA_PASSWORD=$(ZUORA_PASSWORD) -e ZUORA_SUBSCRIPTIONPLANID=$(ZUORA_SUBSCRIPTIONPLANID) \
		-e TESTDIRS=${TESTDIRS} \
		$(IMAGE_PREFIX)/build $@

billing-integration-test: build/$(UPTODATE)
	@mkdir -p $(shell pwd)/.pkg
	DB_CONTAINER="$$(docker run -d -e 'POSTGRES_DB=billing_test' postgres:9.5)"; \
	$(SUDO) docker run $(RM) -ti \
		-v $(shell pwd)/.pkg:/go/pkg \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		-v $(shell pwd)/billing-api/db/migrations:/migrations \
		--workdir /go/src/github.com/weaveworks/service \
		--link "$$DB_CONTAINER":billing-db.weave.local \
		$(IMAGE_PREFIX)/build $@; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$DB_CONTAINER"; \
	exit $$status

flux-nats-tests: build/$(UPTODATE)
	@mkdir -p $(shell pwd)/.pkg
	NATS_CONTAINER="$$(docker run -d nats)"; \
	$(SUDO) docker run $(RM) -ti \
		-v $(shell pwd)/.pkg:/go/pkg \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		-v $(shell pwd)/billing-api/db/migrations:/migrations \
		--workdir /go/src/github.com/weaveworks/service \
		--link "$$NATS_CONTAINER":nats \
		$(IMAGE_PREFIX)/build $@; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$NATS_CONTAINER"; \
	exit $$status

else

$(EXES): build/$(UPTODATE) users/users.pb.go kubectl-service/grpc/kubectl-service.pb.go gcp-service/grpc/gcp-service.pb.go
	go build $(GO_FLAGS) -o $@ ./$(@D)
	$(NETGO_CHECK)

%.pb.go: build/$(UPTODATE)
	protoc -I ./vendor:./$(@D) --gogoslick_out=plugins=grpc:./$(@D) ./$(patsubst %.pb.go,%.proto,$@)

lint: build/$(UPTODATE)
	./tools/lint .

test: build/$(UPTODATE) users/users.pb.go kubectl-service/grpc/kubectl-service.pb.go gcp-service/grpc/gcp-service.pb.go $(MOCK_GOS)
	TESTDIRS=${TESTDIRS} ./tools/test -netgo -no-race

$(MOCK_USERS): build/$(UPTODATE)
	mockgen -destination=$@ github.com/weaveworks/service/users UsersClient \
		&& sed -i'' s,github.com/weaveworks/service/vendor/,, $@

$(MOCK_BILLING_DB): build/$(UPTODATE) $(BILLING_DB)/db.go
	mockgen -destination=$@ github.com/weaveworks/service/$(BILLING_DB) DB

$(MOCK_COMMON_GCP_PARTNER_CLIENT): build/$(UPTODATE)
	mockgen -destination=$@ github.com/weaveworks/service/common/gcp/partner API \
		&& sed -i'' s,github.com/weaveworks/service/vendor/,, $@

$(MOCK_COMMON_GCP_PARTNER_ACCESS): build/$(UPTODATE)
	mockgen -destination=$@ github.com/weaveworks/service/common/gcp/partner Accessor \
		&& sed -i'' s,github.com/weaveworks/service/vendor/,, $@

billing-integration-test: build/$(UPTODATE) $(MOCK_GOS)
	/bin/bash -c "go test -tags 'netgo integration' -timeout 30s $(BILLING_TEST_DIRS)"

flux-nats-tests:
	/bin/bash -c "go test -tags nats -timeout 30s ./flux-api ./flux-api/bus/nats -args -nats-url=nats://nats:4222"

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

users-integration-test: $(USERS_UPTODATE) users/users.pb.go $(MOCK_GOS)
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

pubsub-integration-test:
	PUBSUB_EMU_CONTAINER="$$(docker run --net=host -p 127.0.0.1:8085:8085 -d adilsoncarvalho/gcloud-pubsub-emulator:latest)"; \
	docker run $(RM) \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		--net=host -p 127.0.0.1:1337:1337 \
		--workdir /go/src/github.com/weaveworks/service/common/gcp/pubsub \
		golang:1.8.3-stretch \
		/bin/bash -c "RUN_MANUAL_TEST=1 go test -tags integration -timeout 30s ./..."; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$PUBSUB_EMU_CONTAINER"; \
	exit $$status

kubectl-service-integration-test: kubectl-service/$(UPTODATE) kubectl-service/grpc/kubectl-service.pb.go
	SVC_CONTAINER="$$(docker run -d -p 4887:4772 -p 8887:80 $(IMAGE_PREFIX)/kubectl-service -dry-run=true)"; \
	docker run $(RM) \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		--workdir /go/src/github.com/weaveworks/service/kubectl-service \
		--link "$$SVC_CONTAINER":kubectl-service.weave.local \
		golang:1.8.3-stretch \
		/bin/bash -c "go test -tags integration -timeout 30s ./..."; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$SVC_CONTAINER"; \
	exit $$status

gcp-service-integration-test: gcp-service/$(UPTODATE) gcp-service/grpc/gcp-service.pb.go
	SVC_CONTAINER="$$(docker run -d -p 4888:4772 -p 8888:80 $(IMAGE_PREFIX)/gcp-service -dry-run=true)"; \
	docker run $(RM) \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		--workdir /go/src/github.com/weaveworks/service/gcp-service \
		--link "$$SVC_CONTAINER":gcp-service.weave.local \
		golang:1.8.3-stretch \
		/bin/bash -c "go test -tags integration -timeout 30s ./..."; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$SVC_CONTAINER"; \
	exit $$status

clean:
	$(SUDO) docker rmi $(IMAGE_NAMES) >/dev/null 2>&1 || true
	rm -rf $(UPTODATE_FILES) $(EXES)
	rm -rf billing-aggregator/migrations billing-api/migrations billing-uploader/migrations
	go clean ./...

# For .SECONDEXPANSION docs, see https://www.gnu.org/software/make/manual/html_node/Special-Targets.html
.SECONDEXPANSION:
$(BILLING_EXE): $$(shell find $$(@D) -name '*.go') $(COMMON) $(shell find $(BILLING_DB) -name '*.go') users/users.pb.go
