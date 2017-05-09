PROJECT_NAME := k8s-ec2-srcdst
GOFILES:=$(shell find . -name '*.go' | grep -v -E '(./vendor)')
VERSION?=$(shell git describe --tags --dirty)
IMAGE_TAG:=ottoyiu/${PROJECT_NAME}:${VERSION}

all: clean bin image

image:
	docker build -t ${IMAGE_TAG} .

push_image:
	docker push ${IMAGE_TAG}

bin: bin/linux/${PROJECT_NAME}

bin/%: LDFLAGS=-X github.com/ottoyiu/${PROJECT_NAME}.Version=${VERSION}
bin/%: $(GOFILES)
	mkdir -p $(dir $@)
	CGO_ENABLED=0 GOOS=$(word 1, $(subst /, ,$*)) GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o "$@" github.com/ottoyiu/k8s-ec2-srcdst/cmd/k8s-ec2-srcdst/...

check:
	@find . -name vendor -prune -o -name '*.go' -exec gofmt -s -d {} +
	@go vet $(shell go list ./... | grep -v '/vendor/')
	@go test -v $(shell go list ./... | grep -v '/vendor/')

gofmt:
	@gofmt -w -s pkg/
	@gofmt -w -s cmd/

vendor:
		dep ensure
clean:
		rm -rf bin


.PHONY: all
