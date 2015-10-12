.PHONY: all clean

CLIENT_BUILD_IMAGE=service_client_build
CLIENT_SERVER_IMAGE=registry.weave.local/ui-server
RM=--rm
UISERVERIMAGE_UPTODATE=.ui-server.uptodate
BUILDIMAGE_UPTODATE=.service_client_build.uptodate
JS_FILES=$(shell find ./src -name '*.jsx' -or -name '*.js')

all: $(UISERVERIMAGE_UPTODATE)

buildimage: $(BUILDIMAGE_UPTODATE)

$(BUILDIMAGE_UPTODATE): Dockerfile package.json karma.* webpack.*
	docker build -t $(CLIENT_BUILD_IMAGE) .
	touch $(BUILDIMAGE_UPTODATE)

tests: *
	docker run $(RM) -ti -v $(shell pwd)/src:/home/weave/src \
		-v $(shell pwd)/test:/home/weave/test \
		$(CLIENT_BUILD_IMAGE) npm test

build/app.js: $(BUILDIMAGE_UPTODATE) $(JS_FILES)
	mkdir -p build
	docker run $(RM) -ti -v $(shell pwd)/src:/home/weave/src \
		-v $(shell pwd)/build:/home/weave/build \
		$(CLIENT_BUILD_IMAGE) npm run build

$(UISERVERIMAGE_UPTODATE): build/app.js build/Dockerfile build/index.html
	docker build -t $(CLIENT_SERVER_IMAGE) build/
	touch $@

clean:
	-docker rmi $(CLIENT_SERVER_IMAGE) > /dev/null 2>&1 || true
	rm -f $(UISERVERIMAGE_UPTODATE) build/app.js $(BUILDIMAGE_UPTODATE)

