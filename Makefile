PROJECT_NAME := kubernetes-ec2-srcdst-controller
TAG := $(shell git describe --tags --dirty)
GOFILES:=$(shell find . -name '*.go' | grep -v -E '(./vendor)')

all: build_image

build_image:
	docker build -t ${PROJECT_NAME} .

bin: bin/linux/${PROJECT_NAME}

bin/%: $(GOFILES)
	mkdir -p $(dir $@)
	GOOS=$(word 1, $(subst /, ,$*)) GOARCH=amd64 go build -o $@ github.com/ottoyiu/kubernetes-ec2-srcdst-controller

check:
	@find . -name vendor -prune -o -name '*.go' -exec gofmt -s -d {} +
	@go vet $(shell go list ./... | grep -v '/vendor/')
	@go test -v $(shell go list ./... | grep -v '/vendor/')

vendor:
		dep ensure
clean:
		rm -rf bin


.PHONY: all
