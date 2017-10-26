.DEFAULT: all
.PHONY: all release-bins clean realclean test

DOCKER?=docker
TEST_FLAGS?=

include docker/kubectl.version

# NB because this outputs absolute file names, you have to be careful
# if you're testing out the Makefile with `-W` (pretend a file is
# new); use the full path to the pretend-new file, e.g.,
#  `make -W $PWD/registry/registry.go`
godeps=$(shell go list -f '{{join .Deps "\n"}}' $1 | grep -v /vendor/ | xargs go list -f '{{if not .Standard}}{{ $$dep := . }}{{range .GoFiles}}{{$$dep.Dir}}/{{.}} {{end}}{{end}}')

FLUXD_DEPS:=$(call godeps,./cmd/fluxd)
FLUXSVC_DEPS:=$(call godeps,./cmd/fluxsvc)
FLUXCTL_DEPS:=$(call godeps,./cmd/fluxctl)

MIGRATIONS:=$(shell find service/db/migrations -type f)

IMAGE_TAG:=$(shell ./docker/image-tag)

all: $(GOPATH)/bin/fluxctl $(GOPATH)/bin/fluxd $(GOPATH)/bin/fluxsvc build/.flux.done build/.flux-service.done

release-bins:
	for arch in amd64; do \
		for os in linux darwin; do \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -o "build/fluxctl_"$$os"_$$arch" $(LDFLAGS) -ldflags "-X main.version=$(shell ./docker/image-tag)" ./cmd/fluxctl/; \
		done; \
	done

clean:
	go clean
	rm -rf ./build

realclean: clean
	rm -rf ./cache

test:
	@export PATH="$$PATH:$$PWD/cmd/fluxsvc"; \
	go test ${TEST_FLAGS} $(shell go list ./... | grep -v "^github.com/weaveworks/flux/vendor" | sort -u)

build/migrations.tar: $(MIGRATIONS)
	tar cf $@ service/db/migrations

build/.%.done: docker/Dockerfile.%
	mkdir -p ./build/docker/$*
	cp $^ ./build/docker/$*/
	${DOCKER} build -t quay.io/weaveworks/$* -t quay.io/weaveworks/$*:$(IMAGE_TAG) -f build/docker/$*/Dockerfile.$* ./build/docker/$*
	touch $@

build/.flux.done: build/fluxd build/kubectl
build/.flux-service.done: build/fluxsvc build/migrations.tar

build/fluxd: $(FLUXD_DEPS)
build/fluxd: cmd/fluxd/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ $(LDFLAGS) -ldflags "-X main.version=$(shell ./docker/image-tag)" ./cmd/fluxd

build/fluxsvc: $(FLUXSVC_DEPS)
build/fluxsvc: cmd/fluxsvc/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ $(LDFLAGS) -ldflags "-X main.version=$(shell ./docker/image-tag)" ./cmd/fluxsvc

build/kubectl: cache/kubectl-$(KUBECTL_VERSION) docker/kubectl.version
	cp cache/kubectl-$(KUBECTL_VERSION) $@
	strip $@
	chmod a+x $@

cache/kubectl-$(KUBECTL_VERSION):
	mkdir -p cache
	curl -L -o $@ "https://storage.googleapis.com/kubernetes-release/release/$(KUBECTL_VERSION)/bin/linux/amd64/kubectl"

${GOPATH}/bin/fluxctl: $(FLUXCTL_DEPS)
${GOPATH}/bin/fluxctl: ./cmd/fluxctl/*.go
	go install ./cmd/fluxctl

$(GOPATH)/bin/fluxd: $(FLUXD_DEPS)
$(GOPATH)/bin/fluxd: cmd/fluxd/*.go
	go install ./cmd/fluxd

$(GOPATH)/bin/fluxsvc: $(FLUXSVC_DEPS)
$(GOPATH)/bin/fluxsvc: cmd/fluxsvc/*.go
	go install ./cmd/fluxsvc
