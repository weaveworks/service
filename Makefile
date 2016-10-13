.PHONY: all test users-integration-test clean client-lint images ui-upload
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

# List of exes please
AUTHFE_EXE := authfe/authfe
CONFIGS_EXE := configs/cmd/configs/configs
USERS_EXE := users/cmd/users/users
METRICS_EXE := metrics/metrics
PR_ASSIGNER_EXE := pr-assigner/pr-assigner
EXES = $(AUTHFE_EXE) $(CONFIGS_EXE) $(USERS_EXE) $(METRICS_EXE) $(PROM_RUN_EXE) $(PR_ASSIGNER_EXE)

# And what goes into each exe
COMMON := $(shell find common -name '*.go')
$(AUTHFE_EXE): $(shell find authfe -name '*.go') $(shell find users/client -name '*.go') $(COMMON)
$(CONFIGS_EXE): $(shell find configs -name '*.go') $(COMMON)
$(USERS_EXE): $(shell find users -name '*.go') $(COMMON)
$(METRICS_EXE): $(shell find metrics -name '*.go') $(COMMON)
$(PR_ASSIGNER_EXE): $(shell find pr-assigner -name '*.go') $(COMMON)

# And now what goes into each image
authfe/$(UPTODATE): $(AUTHFE_EXE)
configs/$(UPTODATE): $(CONFIGS_EXE)
users/$(UPTODATE): $(USERS_EXE) $(shell find users -name '*.sql') users/templates/*
metrics/$(UPTODATE): $(METRICS_EXE)
frontend-mt/$(UPTODATE): frontend-mt/default.conf frontend-mt/routes.conf frontend-mt/api.json frontend-mt/pki/scope.weave.works.crt frontend-mt/dhparam.pem
logging/$(UPTODATE): logging/fluent.conf logging/fluent-dev.conf logging/schema_service_events.json
ui-server/client/$(UPTODATE): ui-server/client/package.json ui-server/client/webpack.* ui-server/client/server.js ui-server/client/.eslintrc ui-server/client/.eslintignore ui-server/client/.babelrc
ui-server/$(UPTODATE): ui-server/client/build/index.html
build/$(UPTODATE): build/build.sh
monitoring/grafana/$(UPTODATE): monitoring/grafana/*
monitoring/gfdatasource/$(UPTODATE): monitoring/gfdatasource/*
monitoring/prometheus/$(UPTODATE): monitoring/prometheus/*
monitoring/alertmanager/$(UPTODATE): monitoring/alertmanager/*
pr-assigner/$(UPTODATE): $(PR_ASSIGNER_EXE)

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
	./monitoring/lint
	./tools/shell-lint .

test: build/$(UPTODATE)
	./tools/test -no-go-get

endif

# All the boiler plate for building the client follows:
JS_FILES=$(shell find ui-server/client/src -name '*.jsx' -or -name '*.js')
WEBPACK_DEPS = ui-server/client/$(UPTODATE) $(JS_FILES) ui-server/client/src/html/index.html ui-server/client/.eslintignore ui-server/client/.eslintrc ui-server/client/.babelrc

client-lint: ui-server/client/$(UPTODATE) $(JS_FILES)
	$(SUDO) docker run $(RM) -ti \
		-v $(shell pwd)/ui-server/client/src:/home/weave/src \
		$(IMAGE_PREFIX)/client npm run lint

ui-server/client/build/index.html: $(WEBPACK_DEPS) ui-server/client/webpack.production.config.js
	mkdir -p ui-server/client/build
	$(SUDO) docker run $(RM) -ti \
		-v $(shell pwd)/ui-server/client/src:/home/weave/src \
		-v $(shell pwd)/ui-server/client/build:/home/weave/build \
		$(IMAGE_PREFIX)/client npm run build-production
	cp -p ui-server/client/src/images/* ui-server/client/build

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

ui-upload: ui-server/client/build/index.html
	AWS_ACCESS_KEY_ID=$$UI_BUCKET_KEY_ID \
	AWS_SECRET_ACCESS_KEY=$$UI_BUCKET_KEY_SECRET \
	aws s3 cp ui-server/client/build/ s3://static.weave.works/service-ui/ --recursive --exclude index.html

clean:
	$(SUDO) docker rmi $(IMAGE_NAMES) >/dev/null 2>&1 || true
	rm -rf $(UPTODATE_FILES) $(EXES)
	rm -rf ui-server/client/build
	go clean ./...

