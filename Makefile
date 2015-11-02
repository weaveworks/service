.PHONY: all test app-mapper-integration-test users-integration-test deps clean client-build-image client-tests client-lint
.DEFAULT: all

BUILD_UPTODATE=build/.image.uptodate
BUILD_IMAGE=service_build

APP_MAPPER_UPTODATE=app-mapper/.images.uptodate
APP_MAPPER_EXE=app-mapper/app-mapper
APP_MAPPER_IMAGE=quay.io/weaveworks/app-mapper
APP_MAPPER_DB_IMAGE=weaveworks/app-mapper-db # The DB image is only used in the local environemnt and it's not pushed to Quay

USERS_UPTODATE=users/.images.uptodate
USERS_EXE=users/users
USERS_IMAGE=quay.io/weaveworks/users
USERS_DB_IMAGE=weaveworks/users-db # The DB image is only used in the local environemnt and it's not pushed to Quay

CLIENT_SERVER_UPTODATE=client/.ui-server.uptodate
CLIENT_BUILD_UPTODATE=client/.service_client_build.uptodate
CLIENT_SERVER_IMAGE=quay.io/weaveworks/ui-server
CLIENT_BUILD_IMAGE=service_client_build # only for local use
JS_FILES=$(shell find client/src -name '*.jsx' -or -name '*.js')

FRONTEND_UPTODATE=frontend/.image.uptodate
FRONTEND_IMAGE=quay.io/weaveworks/frontend

MONITORING_UPTODATE=monitoring/.images.uptodate

# If you can use Docker without being root, you can `make SUDO= <target>`
SUDO=sudo -E
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

all: $(APP_MAPPER_UPTODATE) $(USERS_UPTODATE) $(CLIENT_SERVER_UPTODATE) $(FRONTEND_UPTODATE) $(MONITORING_UPTODATE)

$(BUILD_UPTODATE): build/*
	$(DOCKER_HOST_CHECK)
	$(SUDO) docker build -t $(BUILD_IMAGE) build/
	touch $@

$(APP_MAPPER_UPTODATE): app-mapper/Dockerfile $(APP_MAPPER_EXE) app-mapper/db/Dockerfile app-mapper/db/schema.sql
	$(DOCKER_HOST_CHECK)
	$(SUDO) docker build -t $(APP_MAPPER_IMAGE) app-mapper/
	$(SUDO) docker build -t $(APP_MAPPER_DB_IMAGE) app-mapper/db/
	touch $@

$(USERS_UPTODATE): users/Dockerfile users/templates/* $(USERS_EXE) users/db/* users/ca-certificates.crt
	$(DOCKER_HOST_CHECK)
	$(SUDO) docker build -t $(USERS_IMAGE) users/
	$(SUDO) docker build -t $(USERS_DB_IMAGE) users/db/
	touch $@

$(CLIENT_BUILD_UPTODATE): client/Dockerfile client/package.json client/karma.* client/webpack.*
	$(DOCKER_HOST_CHECK)
	$(SUDO) docker build -t $(CLIENT_BUILD_IMAGE) client/
	touch $@

$(CLIENT_SERVER_UPTODATE): client/build/app.js client/build/Dockerfile client/build/index.html
	$(DOCKER_HOST_CHECK)
	$(SUDO) docker build -t $(CLIENT_SERVER_IMAGE) client/build/
	touch $@

$(FRONTEND_UPTODATE): frontend/Dockerfile frontend/default.conf frontend/api.json
	$(DOCKER_HOST_CHECK)
	$(SUDO) docker build -t $(FRONTEND_IMAGE) frontend/
	touch $@

$(MONITORING_UPTODATE):
	make -C monitoring

$(APP_MAPPER_EXE): app-mapper/*.go
$(USERS_EXE): users/*.go users/names/*.go

ifeq ($(BUILD_IN_CONTAINER),true)
$(APP_MAPPER_EXE) $(USERS_EXE): $(BUILD_UPTODATE)
	$(SUDO) docker run $(RM) -v $(shell pwd):/go/src/github.com/weaveworks/service $(BUILD_IMAGE) $@
else
$(APP_MAPPER_EXE) $(USERS_EXE): $(BUILD_UPTODATE)
	go build $(GO_FLAGS) -o $@ ./$(@D)
	$(NETGO_CHECK)
endif

app-mapper-integration-test: $(APP_MAPPER_UPTODATE)
	@app-mapper/script/integration-test.sh

users-integration-test: $(USERS_UPTODATE)
	DB_CONTAINER="$$(docker run -d $(USERS_DB_IMAGE))"; \
	docker run $(RM) \
		-v $(shell pwd):/go/src/github.com/weaveworks/service \
		--workdir /go/src/github.com/weaveworks/service/users \
		--link "$$DB_CONTAINER":users-db.weave.local \
		golang:1.5.1 \
		/bin/bash -c "go get -v -d -t ./... ; go test -tags integration -timeout 30s ./..."; \
	status=$$?; \
	test -n "$(CIRCLECI)" || docker rm -f "$$DB_CONTAINER"; \
	exit $$status

users/ca-certificates.crt: /etc/ssl/certs/ca-certificates.crt
	cp $? $@

client-build-image: $(CLIENT_BUILD_UPTODATE)

ifeq ($(BUILD_IN_CONTAINER),true)
test: $(BUILD_UPTODATE)
	$(SUDO) docker run $(RM) -v $(shell pwd):/go/src/github.com/weaveworks/service \
		-e CIRCLECI -e CIRCLE_BUILD_NUM -e CIRCLE_NODE_TOTAL -e CIRCLE_NODE_INDEX -e COVERDIR \
		$(BUILD_IMAGE) test
else
test: $(BUILD_UPTODATE)
	./tools/test -no-go-get
endif

clean:
	-$(SUDO) docker rmi $(APP_MAPPER_IMAGE) $(APP_MAPPER_DB_IMAGE) $(USERS_IMAGE) \
		$(USERS_DB_IMAGE) $(CLIENT_SERVER_IMAGE) $(CLIENT_BUILD_IMAGE) $(FRONTEND_IMAGE) \
		$(BUILD_IMAGE) >/dev/null 2>&1 || true
	rm -rf $(APP_MAPPER_EXE) $(APP_MAPPER_UPTODATE) $(USERS_EXE) $(USERS_UPTODATE) \
		$(CLIENT_BUILD_UPTODATE) $(CLIENT_SERVER_UPTODATE) $(FRONTEND_UPTODATE) \
		$(BUILD_UPTODATE) client/build/app.js $(APP_MAPPER_EXE)$(IN_CONTAINER) \
		$(USERS_EXE)$(IN_CONTAINER)
	go clean ./...

deps:
	go get \
		github.com/golang/lint/golint \
		github.com/fzipp/gocyclo \
		github.com/mattn/goveralls \
		github.com/kisielk/errcheck

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



