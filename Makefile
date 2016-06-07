.PHONY: all test app-mapper-integration-test users-integration-test deps clean client-build-image client-tests client-lint
.DEFAULT: all

BUILD_UPTODATE=build/.image.uptodate
BUILD_IMAGE=service_build

AUTHFE_UPTODATE=authfe/.images.uptodate
AUTHFE_EXE=authfe/authfe
AUTHFE_IMAGE=quay.io/weaveworks/authfe

USERS_UPTODATE=users/.images.uptodate
USERS_EXE=users/users
USERS_IMAGE=quay.io/weaveworks/users
USERS_DB_IMAGE=weaveworks/users-db # The DB image is only used in the local environment and it's not pushed to Quay
USERS_DB_MIGRATE_EXE=users/db/migrate

METRICS_UPTODATE=metrics/.uptodate
METRICS_EXE=metrics/metrics
METRICS_IMAGE=quay.io/weaveworks/metrics

JSON_BUILDER_UPTODATE=launch-generator/.uptodate
JSON_BUILDER_IMAGE=quay.io/weaveworks/launch-generator

CLIENT_SERVER_UPTODATE=client/.ui-server.uptodate
CLIENT_BUILD_UPTODATE=client/.service_client_build.uptodate
CLIENT_SERVER_IMAGE=quay.io/weaveworks/ui-server
CLIENT_BUILD_IMAGE=service_client_build # only for local use
JS_FILES=$(shell find client/src -name '*.jsx' -or -name '*.js')

FRONTEND_MT_UPTODATE=frontend-mt/.image.uptodate
FRONTEND_MT_IMAGE=quay.io/weaveworks/frontend-mt

MONITORING_UPTODATE=monitoring/.images.uptodate

KUBEDIFF_UPTODATE=kubediff/.image.uptodate
KUBEDIFF_IMAGE=quay.io/weaveworks/kubediff
PROM_RUN_EXE=vendor/github.com/tomwilkie/prom-run/prom-run

# If you can use Docker without being root, you can `make SUDO= <target>`
SUDO=$(shell (echo "$$DOCKER_HOST" | grep "tcp://" >/dev/null) || echo "sudo -E")
BUILD_IN_CONTAINER=true
RM=--rm
GO_FLAGS=-ldflags "-extldflags \"-static\" -linkmode=external" -tags netgo
DOCKER_HOST_CHECK=@if echo "$$DOCKER_HOST" | grep "127.0.0.1" >/dev/null; then \
	echo "DOCKER_HOST is set to \"$$DOCKER_HOST\"!"; \
	echo "If you are trying to build for dev/prod, this is probably a mistake."; \
	while true; do \
		read -p "Are you sure you want to continue? " yn; \
		case $$yn in \
			yes) break;; \
			no) exit 1;; \
			*) echo "Please type 'yes' or 'no'.";; \
		esac; \
	done; \
fi
NETGO_CHECK=@strings $@ | grep cgo_stub\\\.go >/dev/null || { \
	rm $@; \
	echo "\nYour go standard library was built without the 'netgo' build tag."; \
	echo "To fix that, run"; \
	echo "    sudo go clean -i net"; \
	echo "    sudo go install -tags netgo std"; \
	false; \
}

all: $(AUTHFE_UPTODATE) $(USERS_UPTODATE) $(CLIENT_SERVER_UPTODATE) $(MONITORING_UPTODATE) $(METRICS_UPTODATE) $(JSON_BUILDER_UPTODATE) $(FRONTEND_MT_UPTODATE) $(KUBEDIFF_UPTODATE)


$(BUILD_UPTODATE): build/*
	$(DOCKER_HOST_CHECK)
	$(SUDO) docker build -t $(BUILD_IMAGE) build/
	touch $@

$(AUTHFE_UPTODATE): authfe/Dockerfile $(AUTHFE_EXE)
	$(DOCKER_HOST_CHECK)
	$(SUDO) docker build -t $(AUTHFE_IMAGE) authfe/
	touch $@

$(USERS_UPTODATE): $(USERS_EXE) $(shell find users -name '*.sql') users/Dockerfile $(USERS_DB_MIGRATE_EXE)
	$(DOCKER_HOST_CHECK)
	$(SUDO) docker build -t $(USERS_IMAGE) users/
	$(SUDO) docker build -t $(USERS_DB_IMAGE) users/db/
	touch $@

$(METRICS_UPTODATE): metrics/Dockerfile $(METRICS_EXE)
	$(DOCKER_HOST_CHECK)
	$(SUDO) docker build -t $(METRICS_IMAGE) metrics/
	touch $@

$(JSON_BUILDER_UPTODATE): launch-generator/Dockerfile launch-generator/src/*.js launch-generator/package.json
	$(DOCKER_HOST_CHECK)
	$(SUDO) docker build -t $(JSON_BUILDER_IMAGE) launch-generator/
	touch $@

$(CLIENT_BUILD_UPTODATE): client/Dockerfile client/package.json client/webpack.*
	$(DOCKER_HOST_CHECK)
	$(SUDO) docker build -t $(CLIENT_BUILD_IMAGE) client/
	touch $@

$(CLIENT_SERVER_UPTODATE): client/build/app.js client/src/Dockerfile client/src/html/index.html
	$(DOCKER_HOST_CHECK)
	cp client/src/Dockerfile client/build/
	$(SUDO) docker build -t $(CLIENT_SERVER_IMAGE) client/build/
	touch $@

$(FRONTEND_MT_UPTODATE): frontend-mt/Dockerfile frontend-mt/default.conf frontend-mt/api.json frontend-mt/pki/scope.weave.works.crt frontend-mt/dhparam.pem
	$(DOCKER_HOST_CHECK)
	$(SUDO) docker build -t $(FRONTEND_MT_IMAGE) frontend-mt/
	touch $@

$(MONITORING_UPTODATE):
	make -C monitoring

$(KUBEDIFF_UPTODATE): kubediff/Dockerfile $(PROM_RUN_EXE)
	$(DOCKER_HOST_CHECK)
	cp $(PROM_RUN_EXE) kubediff/
	$(SUDO) docker build -t $(KUBEDIFF_IMAGE) kubediff
	touch $@

$(AUTHFE_EXE): $(shell find authfe -name '*.go')
$(USERS_EXE): $(shell find users -name '*.go')
$(METRICS_EXE): $(shell find metrics -name '*.go')
$(PROM_RUN_EXE): $(shell find ./vendor/github.com/tomwilkie/prom-run/.)
$(USERS_DB_MIGRATE_EXE): $(shell find ./vendor/github.com/mattes/migrate/.)

ifeq ($(BUILD_IN_CONTAINER),true)

$(AUTHFE_EXE) $(USERS_EXE) $(USERS_DB_MIGRATE_EXE) $(METRICS_EXE) $(PROM_RUN_EXE) lint test: $(BUILD_UPTODATE)
	$(SUDO) docker run $(RM) -ti -v $(shell pwd):/go/src/github.com/weaveworks/service \
		-e CIRCLECI -e CIRCLE_BUILD_NUM -e CIRCLE_NODE_TOTAL -e CIRCLE_NODE_INDEX -e COVERDIR \
		$(BUILD_IMAGE) $@

else

$(AUTHFE_EXE) $(USERS_EXE): $(BUILD_UPTODATE)
	go build $(GO_FLAGS) -o $@ ./$(@D)
	$(NETGO_CHECK)

$(METRICS_EXE): $(BUILD_UPTODATE)
	go build $(GO_FLAGS) -o $@ ./$(@D)

$(USERS_DB_MIGRATE_EXE): $(BUILD_UPTODATE)
	go build $(GO_FLAGS) -o $@ ./vendor/github.com/mattes/migrate

$(PROM_RUN_EXE): $(BUILD_UPTODATE)
	go build $(GO_FLAGS) -o $@ ./vendor/github.com/tomwilkie/prom-run

lint: $(BUILD_UPTODATE)
	./tools/lint .
	./k8s/kubelint --noversions ./k8s/local
	./k8s/kubelint --nonamespaces ./k8s/dev ./k8s/prod
	promtool check-rules ./monitoring/prometheus/alert.rules

test: $(BUILD_UPTODATE)
	./tools/test -no-go-get

endif

users-integration-test: $(USERS_UPTODATE)
	DB_CONTAINER="$$(docker run -d $(USERS_DB_IMAGE))"; \
	docker run $(RM) \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		--workdir /go/src/github.com/weaveworks/service/users \
		--link "$$DB_CONTAINER":users-db.weave.local \
		golang:1.6.2 \
		/bin/bash -c "go test -tags integration -timeout 30s ./..."; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$DB_CONTAINER"; \
	exit $$status

client-build-image: $(CLIENT_BUILD_UPTODATE)

clean:
	# Don't remove the build images, just remove the marker files.
	-$(SUDO) docker rmi \
		$(USERS_IMAGE)  $(USERS_DB_MIGRATE_EXE) $(USERS_DB_IMAGE) \
		$(CLIENT_SERVER_IMAGE) $(FRONTEND_MT_IMAGE) \
		$(METRICS_IMAGE) >/dev/null 2>&1 || true
	rm -rf $(USERS_EXE) $(USERS_UPTODATE) \
		$(JSON_BUILDER_UPTODATE) \
		$(METRICS_EXE) $(METRICS_UPTODATE) \
		$(CLIENT_SERVER_UPTODATE) $(FRONTEND_MT_IMAGE) client/build/app.js \
		$(BUILD_UPTODATE) $(CLIENT_BUILD_UPTODATE)
	go clean ./...
	make -C monitoring clean

client-tests: $(CLIENT_BUILD_UPTODATE)
	$(SUDO) docker run $(RM) -ti -v $(shell pwd)/client/src:/home/weave/src \
		-v $(shell pwd)/client/test:/home/weave/test \
		$(CLIENT_BUILD_IMAGE) npm test

client-lint: $(CLIENT_BUILD_UPTODATE) $(JS_FILES)
	$(SUDO) docker run $(RM) -ti -v $(shell pwd)/client/src:/home/weave/src \
		-v $(shell pwd)/client/test:/home/weave/test \
		$(CLIENT_BUILD_IMAGE) npm run lint

client/build/app.js: $(CLIENT_BUILD_UPTODATE) $(JS_FILES)
	mkdir -p client/build
	$(SUDO) docker run $(RM) -ti -v $(shell pwd)/client/src:/home/weave/src \
		-v $(shell pwd)/client/build:/home/weave/build \
		$(CLIENT_BUILD_IMAGE) npm run build
	cp client/src/images/* client/build/


