.PHONY: all test users-integration-test clean client-build-image client-tests client-lint images
.DEFAULT_GOAL := all

# Boiler plate for bulding Docker containers.
# All this must go at top of file I'm afraid.
IMAGE_PREFIX := quay.io/weaveworks
IMAGE_TAG := $(shell ./image-tag)
UPTODATE := .uptodate
UPTODATE_FILES =
IMAGE_NAMES =

# Building Docker images is now automated.  The convention is every directory
# with a Dockerfile in it builds an image calls quay.io/weaveworks/<dirname>.
# Dependancies (ie things that go in the image) still need to be explicitly
# declared.
define DOCKER_IMAGE_template
$(1)/$(UPTODATE): $(1)/Dockerfile
	$(SUDO) docker build -t $(IMAGE_PREFIX)/$(shell basename $(1)) $(1)/
	$(SUDO) docker tag $(IMAGE_PREFIX)/$(shell basename $(1)) $(IMAGE_PREFIX)/$(shell basename $(1)):$(IMAGE_TAG)
	touch $(1)/$(UPTODATE)

UPTODATE_FILES += $(1)/$(UPTODATE)
IMAGE_NAMES += $(IMAGE_PREFIX)/$(shell basename $(1))
endef

# Get a list of directories container Dockerfiles, and run DOCKER_IMAGE on all
# of them.
DOCKER_IMAGE_DIRS=$(shell find * -type f -name Dockerfile ! -path "tools/*" ! -path "vendor/*" | xargs -n1 dirname)
$(foreach dir,$(DOCKER_IMAGE_DIRS),$(eval $(call DOCKER_IMAGE_template,$(dir))))

images:
	$(info $(IMAGE_NAMES))

all: $(UPTODATE_FILES)

# List of exes please
AUTHFE_EXE := authfe/authfe
USERS_EXE := users/users
METRICS_EXE := metrics/metrics
PROM_RUN_EXE := kubediff/vendor/github.com/tomwilkie/prom-run/prom-run
EXES = $(AUTHFE_EXE) $(USERS_EXE) $(METRICS_EXE) $(PROM_RUN_EXE)

# And what goes into each exe
COMMON := $(shell find common -name '*.go')
$(AUTHFE_EXE): $(shell find authfe -name '*.go') $(COMMON)
$(USERS_EXE): $(shell find users -name '*.go') $(COMMON)
$(METRICS_EXE): $(shell find metrics -name '*.go') $(COMMON)
$(PROM_RUN_EXE): $(shell find ./kubediff/vendor/github.com/tomwilkie/prom-run/ -name '*.go')

# And now what goes into each image
authfe/$(UPTODATE): $(AUTHFE_EXE)
users/$(UPTODATE): $(USERS_EXE) $(shell find users -name '*.sql') users/templates/*
metrics/$(UPTODATE): $(METRICS_EXE)
launch-generator/$(UPTODATE): launch-generator/src/*.js launch-generator/package.json
kubediff/$(UPTODATE): $(PROM_RUN_EXE)
frontend-mt/$(UPTODATE): frontend-mt/default.conf frontend-mt/routes.conf frontend-mt/api.json frontend-mt/pki/scope.weave.works.crt frontend-mt/dhparam.pem
logging/$(UPTODATE): logging/fluent.conf logging/fluent-dev.conf logging/schema_service_events.json
client/$(UPTODATE): client/package.json client/webpack.* client/server.js
ui-server/$(UPTODATE): ui-server/build/app.js
build/$(UPTODATE): build/build.sh
monitoring/grafana/$(UPTODATE): monitoring/grafana/*
monitoring/gfdatasource/$(UPTODATE): monitoring/gfdatasource/*
monitoring/prometheus/$(UPTODATE): monitoring/prometheus/*

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

$(EXES) lint test: build/$(UPTODATE)
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

lint: build/$(UPTODATE)
	./tools/lint .
	# This mapping of cluster to lint options is duplicated in 'rolling-update'.
	./k8s/kubelint --noversions ./k8s/local
	./k8s/kubelint ./k8s/dev ./k8s/prod
	./monitoring/lint

test: build/$(UPTODATE)
	./tools/test -no-go-get

endif

# All the boiler plate for building the client follows:
JS_FILES=$(shell find client/src -name '*.jsx' -or -name '*.js')

client-tests: client/$(UPTODATE) $(JS_FILES)
	$(SUDO) docker run $(RM) -ti -v $(shell pwd)/client/src:/home/weave/src \
		-v $(shell pwd)/client/test:/home/weave/test \
		$(IMAGE_PREFIX)/client npm test

client-lint: client/$(UPTODATE) $(JS_FILES)
	$(SUDO) docker run $(RM) -ti -v $(shell pwd)/client/src:/home/weave/src \
		-v $(shell pwd)/client/test:/home/weave/test \
		$(IMAGE_PREFIX)/client npm run lint

client/build/app.js: client/$(UPTODATE) $(JS_FILES) client/src/html/index.html
	mkdir -p client/build
	$(SUDO) docker run $(RM) -ti -v $(shell pwd)/client/src:/home/weave/src \
		-v $(shell pwd)/client/build:/home/weave/build \
		$(IMAGE_PREFIX)/client npm run build
	cp -p client/src/images/* client/build/

ui-server/build/app.js: client/build/app.js
	mkdir -p $(@D)
	install  client/build/* $(@D)/

# Test and misc stuff
users-integration-test: $(USERS_UPTODATE)
	DB_CONTAINER="$$(docker run -d quay.io/weaveworks/users-db)"; \
	docker run $(RM) \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		-v $(shell pwd)/users/db/migrations:/migrations \
		--workdir /go/src/github.com/weaveworks/service/users \
		--link "$$DB_CONTAINER":users-db.weave.local \
		golang:1.6.2 \
		/bin/bash -c "go test -tags integration -timeout 30s ./..."; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$DB_CONTAINER"; \
	exit $$status

clean:
	$(SUDO) docker rmi $(IMAGE_NAMES) >/dev/null 2>&1 || true
	rm -rf $(UPTODATE_FILES) $(EXES)
	rm -rf client/build ui-server/build
	go clean ./...


