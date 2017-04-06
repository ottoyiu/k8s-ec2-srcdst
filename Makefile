PROJECT_NAME := kubernetes-ec2-srcdst-controller
GOFILES:=$(shell find . -name '*.go' | grep -v -E '(./vendor)')
VERSION?=$(shell git describe --tags --dirty)
IMAGE_TAG:=ottoyiu/${PROJECT_NAME}:${VERSION}

all: clean bin image

image:
	docker build -t ${IMAGE_TAG} .

push_image:
	docker push ${IMAGE_TAG}

bin: bin/linux/${PROJECT_NAME}

bin/%: LDFLAGS=-X main.Version=${VERSION}
bin/%: $(GOFILES)
	mkdir -p $(dir $@)
	CGO_ENABLED=0 GOOS=$(word 1, $(subst /, ,$*)) GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $@ github.com/ottoyiu/kubernetes-ec2-srcdst-controller

check:
	@find . -name vendor -prune -o -name '*.go' -exec gofmt -s -d {} +
	@go vet $(shell go list ./... | grep -v '/vendor/')
	@go test -v $(shell go list ./... | grep -v '/vendor/')

vendor:
		dep ensure
clean:
		rm -rf bin


.PHONY: all
