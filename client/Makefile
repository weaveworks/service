CLIENT_BUILD_IMAGE=service_client_build
CLIENT_SERVER_IMAGE=service_client_server

buildimage: Dockerfile package.json karma.* webpack.*
	docker build -t $(CLIENT_BUILD_IMAGE) .

tests: *
	docker run -ti -v $(shell pwd)/src:/home/weave/src \
		-v $(shell pwd)/test:/home/weave/test \
		$(CLIENT_BUILD_IMAGE) npm test

build/app.js:
	mkdir -p build
	docker run -ti -v $(shell pwd)/src:/home/weave/src \
		-v $(shell pwd)/build:/home/weave/build \
		$(CLIENT_BUILD_IMAGE) npm run-script build

image.tar: build/app.js build/Dockerfile build/index.html
	docker build -t $(CLIENT_SERVER_IMAGE) build/
	docker save $(CLIENT_SERVER_IMAGE):latest > $@

clean:
	rm -f image.tar build/app.js
